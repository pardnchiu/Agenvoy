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
	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
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
		Name:       "search_conversation_history",
		ReadOnly:   true,
		Concurrent: true,
		Description: "Search this session's past messages by keyword and semantic similarity.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Search text (e.g. 'redis TTL', 'bwrap sandbox decision').",
				},
				"time_range": map[string]any{
					"type":        "string",
					"description": "Keyword time window. Semantic match ignores this. Widen only if empty.",
					"enum":        utils.Keys(historyTimeRanges),
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
				TimeRange string `json:"time_range"`
				Limit     int    `json:"limit"`
				// avoid small agent like 4.1 be stupid to call with different parameter name
				Query string `json:"query"`
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
			return searchConversationHistoryHandler(ctx, sessionId, keyword, params.TimeRange, limit)
		},
	})
}

func searchConversationHistoryHandler(ctx context.Context, sessionID, keyword, timeRange string, limit int) (string, error) {
	db := torii.DB(torii.DBSessionHist)
	allKeys := db.Keys(sessionID + ":*")
	if len(allKeys) == 0 {
		return "no history found", nil
	}

	keyIdx := make(map[string]int, len(allKeys))
	for i, k := range allKeys {
		keyIdx[k] = i
	}

	_, maxHistory := sessionManager.GetHistory(sessionID)
	skip := min(len(maxHistory)+1, len(allKeys))
	excludeKeys := make(map[string]struct{}, skip)
	if skip > 0 {
		for _, k := range allKeys[len(allKeys)-skip:] {
			excludeKeys[k] = struct{}{}
		}
	}

	searchKeys := allKeys
	if skip > 0 {
		searchKeys = allKeys[:len(allKeys)-skip]
	}

	perSource := max(1, limit/2)

	var afterNano int64
	if d, ok := historyTimeRanges[timeRange]; ok {
		afterNano = time.Now().Add(-d).UnixNano()
	}

	hits := mergeHits(
		keywordHits(db, searchKeys, keyword, afterNano, perSource),
		semanticHits(ctx, db, sessionID, keyword, excludeKeys, perSource),
	)
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
		start := idx - historyWindowBefore
		if start < 0 {
			start = 0
		}
		end := idx + historyWindowAfter
		if end > len(allKeys)-1 {
			end = len(allKeys) - 1
		}
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
