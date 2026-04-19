package file

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	toriidb "github.com/pardnchiu/ToriiDB/core/store"
	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
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

func registSearchHistory() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "search_history",
		ReadOnly:    true,
		Description: "Search the current session's conversation history. Runs literal keyword match (respects time_range) and semantic vector search (cosine top-K, ignores time_range) in parallel; each source caps at limit/2 hits, then merges and dedupes. Each hit is expanded with the 2 preceding and 1 following messages as context. Windows from adjacent hits merge into contiguous segments separated by blank lines. Messages already in the current LLM context are excluded (also from context expansion). Output entries are prefixed with RFC3339 timestamps.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Query text. Used for both literal substring match and semantic embedding query.",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Fallback alias for keyword. Use keyword when possible.",
				},
				"time_range": map[string]any{
					"type":        "string",
					"enum":        []string{"1d", "7d", "1m", "1y"},
					"description": "Time range filter applied only to the keyword portion. 1d=1 day, 7d=7 days, 1m=30 days, 1y=365 days. Semantic portion ignores this to preserve cosine ranking. Context window expansion is also unconstrained to preserve scene completeness. Start with 1d; expand only if no results.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"enum":        []int{8, 16, 32},
					"description": "Hit cap per source (keyword and semantic each take up to limit/2). Default 8. Actual output exceeds limit because each hit expands with 2 preceding + 1 following messages as context (overlapping windows merge into contiguous segments).",
				},
			},
			"required": []string{"keyword"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Keyword   string `json:"keyword"`
				Query     string `json:"query"`
				TimeRange string `json:"time_range"`
				Limit     int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if params.Keyword == "" {
				params.Keyword = params.Query
			}
			return searchHistory(ctx, e.SessionID, params.Keyword, params.TimeRange, params.Limit)
		},
	})
}

func searchHistory(ctx context.Context, sessionID, keyword, timeRange string, limit int) (string, error) {
	switch limit {
	case 8, 16, 32:
	default:
		limit = 8
	}
	if sessionID == "" {
		return "", fmt.Errorf("sessionID is required")
	}
	if keyword == "" {
		return "", fmt.Errorf("keyword is required")
	}

	db := store.DB(store.DBSessionHist)
	allKeys := db.Keys(sessionID + ":*")
	if len(allKeys) == 0 {
		return "no history found for current session", nil
	}

	keyIdx := make(map[string]int, len(allKeys))
	for i, k := range allKeys {
		keyIdx[k] = i
	}

	_, maxHistory := sessionManager.GetHistory(sessionID)
	skip := len(maxHistory) + 1
	if skip > len(allKeys) {
		skip = len(allKeys)
	}

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

	perSource := limit / 2
	if perSource < 1 {
		perSource = 1
	}

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
