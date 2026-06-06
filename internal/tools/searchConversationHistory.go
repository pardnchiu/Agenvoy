package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	toriidb "github.com/pardnchiu/ToriiDB/core/store"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	historyStore "github.com/pardnchiu/agenvoy/internal/session/history/store"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

type historyHit struct {
	Key     string
	TS      int64
	Role    string
	Content string
}

const (
	historyWindowBefore = 2
	historyWindowAfter  = 1
)

var historyTimeRanges = map[string]time.Duration{
	"1d": 24 * time.Hour,
	"7d": 7 * 24 * time.Hour,
	"1m": 30 * 24 * time.Hour,
	"1y": 365 * 24 * time.Hour,
}

func registSearchConversationHistory() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "search_chat_history",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: "Search this session's past messages. mode=keyword for exact match across full history; mode=semantic for meaning-based match. Use for prior conversation references, named entity lookups (call first, then search_web), or theme recall. Extract the core noun as keyword.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Search text (e.g. 'redis TTL', 'bwrap sandbox decision').",
				},
				"mode": map[string]any{
					"type":        "string",
					"description": "keyword: FTS across full history including archive. semantic: vector similarity in recent messages.",
					"enum":        []string{"keyword", "semantic"},
					"default":     "semantic",
				},
				"time_range": map[string]any{
					"type":        "string",
					"description": "Time window filter. Applies to both modes.",
					"enum":        go_pkg_utils.GetKeys(historyTimeRanges),
					"default":     "1d",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Hit cap per source. Output exceeds limit after context expansion.",
					"enum":        []int{8, 16, 32},
					"default":     8,
				},
			},
			"required": []string{
				"keyword",
			},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Keyword   string `json:"keyword"`
				Mode      string `json:"mode"`
				TimeRange string `json:"time_range"`
				Limit     int    `json:"limit"`
				Query     string `json:"query"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			sessionId := e.SessionID
			if sessionId == "" {
				return "", fmt.Errorf("session not exist")
			}

			keyword := strings.TrimSpace(params.Keyword)
			if keyword == "" {
				keyword = params.Query
			}
			if keyword == "" {
				return "", fmt.Errorf("keyword is required")
			}

			mode := strings.TrimSpace(params.Mode)
			if mode != "keyword" {
				mode = "semantic"
			}

			timeRange := strings.TrimSpace(params.TimeRange)
			switch timeRange {
			case "1d", "7d", "1m", "1y":
			default:
				timeRange = "1d"
			}

			limit := params.Limit
			switch limit {
			case 8, 16, 32:
			default:
				limit = 8
			}

			if mode == "keyword" {
				return keywordHandler(ctx, sessionId, keyword, timeRange, limit)
			}
			return semanticHandler(ctx, sessionId, keyword, timeRange, limit)
		},
	})
}

func keywordHandler(_ context.Context, sessionID, keyword, timeRange string, limit int) (string, error) {
	var sb strings.Builder

	reults, err := historyStore.Search(sessionID, keyword, timeRange, limit)
	if err == nil && len(reults) > 0 {
		sb.WriteString("[archive]\n")
		for _, r := range reults {
			tsStr := time.Unix(0, r.Timestamp).Format(time.RFC3339)
			sb.WriteString(fmt.Sprintf("[%s · %s] %s\n", tsStr, r.Role, r.Content))
		}
	}

	db := torii.DB(torii.DBSessionHist)
	allKeys := db.Keys(sessionID + ":*")
	if len(allKeys) > 0 {
		var afterNano int64
		if d, ok := historyTimeRanges[timeRange]; ok {
			afterNano = time.Now().Add(-d).UnixNano()
		}
		recent := keywordHits(db, allKeys, keyword, afterNano, limit)
		if len(recent) > 0 {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("[recent]\n")
			for _, h := range recent {
				tsStr := time.Unix(0, h.TS).Format(time.RFC3339)
				sb.WriteString(fmt.Sprintf("[%s · %s] %s\n", tsStr, h.Role, h.Content))
			}
		}
	}

	if sb.Len() == 0 {
		return fmt.Sprintf("no matches with keyword: %s", keyword), nil
	}
	return sb.String(), nil
}

func semanticHandler(ctx context.Context, sessionID, keyword, timeRange string, limit int) (string, error) {
	db := torii.DB(torii.DBSessionHist)
	allKeys := db.Keys(sessionID + ":*")
	if len(allKeys) == 0 {
		return "no history found", nil
	}

	keyIdx := make(map[string]int, len(allKeys))
	for i, k := range allKeys {
		keyIdx[k] = i
	}

	_, maxHistory := sessionHistory.Get(sessionID)
	skip := min(len(maxHistory)+1, len(allKeys))
	excludeKeys := make(map[string]struct{}, skip)
	if skip > 0 {
		for _, k := range allKeys[len(allKeys)-skip:] {
			excludeKeys[k] = struct{}{}
		}
	}

	hits := semanticHits(ctx, db, sessionID, keyword, excludeKeys, limit)
	if len(hits) == 0 {
		return fmt.Sprintf("no matches with keyword: %s", keyword), nil
	}

	expanded := expandWindows(hits, allKeys, keyIdx, excludeKeys)
	if len(expanded) == 0 {
		return fmt.Sprintf("no matches with keyword: %s", keyword), nil
	}

	return formatSegments(db, allKeys, expanded), nil
}

func keywordHits(db *toriidb.Session, keys []string, keyword string, afterNano int64, cap int) []historyHit {
	lower := strings.ToLower(keyword)
	out := make([]historyHit, 0, cap)

	for i := len(keys) - 1; i >= 0; i-- {
		key := keys[i]
		ts, ok := parseKeyTS(key)
		if !ok {
			continue
		}
		if afterNano > 0 && ts < afterNano {
			break
		}

		entry, ok := db.Get(key)
		if !ok {
			continue
		}
		val := entry.Value()
		if !strings.Contains(strings.ToLower(val), lower) {
			continue
		}

		hit, ok := decodeHit(key, ts, val)
		if !ok {
			continue
		}
		out = append(out, hit)
		if len(out) >= cap {
			break
		}
	}
	return out
}

func semanticHits(ctx context.Context, db *toriidb.Session, sessionID, keyword string, exclude map[string]struct{}, cap int) []historyHit {
	hits, err := db.VSearch(ctx, keyword, sessionID+":*", cap+len(exclude))
	if err != nil {
		return nil
	}

	out := make([]historyHit, 0, cap)
	for _, key := range hits {
		if _, skip := exclude[key]; skip {
			continue
		}
		ts, ok := parseKeyTS(key)
		if !ok {
			continue
		}
		entry, ok := db.Get(key)
		if !ok {
			continue
		}
		hit, ok := decodeHit(key, ts, entry.Value())
		if !ok {
			continue
		}
		out = append(out, hit)
		if len(out) >= cap {
			break
		}
	}
	return out
}

func mergeHits(primary, secondary []historyHit) []historyHit {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	out := make([]historyHit, 0, len(primary)+len(secondary))

	for _, h := range primary {
		if _, ok := seen[h.Key]; ok {
			continue
		}
		seen[h.Key] = struct{}{}
		out = append(out, h)
	}
	for _, h := range secondary {
		if _, ok := seen[h.Key]; ok {
			continue
		}
		seen[h.Key] = struct{}{}
		out = append(out, h)
	}
	return out
}

func expandWindows(hits []historyHit, allKeys []string, keyIdx map[string]int, exclude map[string]struct{}) []int {
	set := make(map[int]struct{}, len(hits)*(historyWindowBefore+historyWindowAfter+1))
	for _, h := range hits {
		idx, ok := keyIdx[h.Key]
		if !ok {
			continue
		}
		start := max(idx-historyWindowBefore, 0)
		end := min(idx+historyWindowAfter, len(allKeys)-1)
		for i := start; i <= end; i++ {
			if _, skip := exclude[allKeys[i]]; skip {
				continue
			}
			set[i] = struct{}{}
		}
	}

	out := make([]int, 0, len(set))
	for i := range set {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

func formatSegments(db *toriidb.Session, allKeys []string, idxs []int) string {
	var sb strings.Builder
	prevIdx := -1
	first := true
	for _, i := range idxs {
		if !first && i != prevIdx+1 {
			sb.WriteString("\n")
		}
		prevIdx = i

		key := allKeys[i]
		entry, ok := db.Get(key)
		if !ok {
			continue
		}
		ts, ok := parseKeyTS(key)
		if !ok {
			continue
		}
		hit, ok := decodeHit(key, ts, entry.Value())
		if !ok {
			continue
		}
		first = false
		tsStr := time.Unix(0, hit.TS).Format(time.RFC3339)
		sb.WriteString(fmt.Sprintf("[%s · %s] %s\n", tsStr, hit.Role, hit.Content))
	}
	return sb.String()
}

func parseKeyTS(key string) (int64, bool) {
	idx := strings.LastIndexByte(key, ':')
	if idx < 0 {
		return 0, false
	}
	ts, err := strconv.ParseInt(key[idx+1:], 10, 64)
	if err != nil {
		return 0, false
	}
	return ts, true
}

func decodeHit(key string, ts int64, val string) (historyHit, bool) {
	var msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(val), &msg); err != nil {
		return historyHit{}, false
	}
	if strings.TrimSpace(msg.Content) == "" {
		return historyHit{}, false
	}
	return historyHit{Key: key, TS: ts, Role: msg.Role, Content: msg.Content}, true
}
