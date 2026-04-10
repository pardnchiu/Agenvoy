package file

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type messageHistory struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

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
		Description: "Search the current session's conversation history by keyword. Returns full message entries (role and content) that contain the keyword. Supports time range filtering.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Keyword to search for (case-insensitive, literal string match)",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Fallback alias for keyword. Use keyword when possible.",
				},
				"time_range": map[string]any{
					"type":        "string",
					"enum":        []string{"1d", "7d", "1m", "1y"},
					"description": "Time range filter: 1d=1 day, 7d=7 days, 1m=30 days, 1y=365 days. Start with 1d; expand to 7d or beyond only if no results.",
				},
			},
			"required": []string{"keyword"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Keyword   string `json:"keyword"`
				Query     string `json:"query"`
				TimeRange string `json:"time_range"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if params.Keyword == "" {
				params.Keyword = params.Query
			}
			return searchHistory(e.SessionID, params.Keyword, params.TimeRange)
		},
	})
}

func searchHistory(sessionID, keyword, timeRange string) (string, error) {
	const limit = 8
	if sessionID == "" {
		return "", fmt.Errorf("sessionID is required")
	}
	if keyword == "" {
		return "", fmt.Errorf("keyword is required")
	}

	db := store.DB(store.DBSessionHist)
	keys := db.Keys(sessionID + ":*")
	if len(keys) == 0 {
		return "no history found for current session", nil
	}

	var afterNano int64
	if d, ok := historyTimeRanges[timeRange]; ok {
		afterNano = time.Now().Add(-d).UnixNano()
	}

	lower := strings.ToLower(keyword)
	matches := make([]messageHistory, 0, limit)

	for i := len(keys) - 1; i >= 0; i-- {
		key := keys[i]

		if afterNano > 0 {
			if idx := strings.LastIndexByte(key, ':'); idx >= 0 {
				if ts, err := strconv.ParseInt(key[idx+1:], 10, 64); err == nil && ts < afterNano {
					break
				}
			}
		}

		entry, ok := db.Get(key)
		if !ok {
			continue
		}
		if !strings.Contains(strings.ToLower(entry.Value), lower) {
			continue
		}

		var msg messageHistory
		if err := json.Unmarshal([]byte(entry.Value), &msg); err != nil {
			continue
		}
		matches = append(matches, msg)
		if len(matches) >= limit {
			break
		}
	}

	if len(matches) == 0 {
		return fmt.Sprintf("no matches with keyword: %s", keyword), nil
	}

	var sb strings.Builder
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", m.Role, m.Content))
	}
	return sb.String(), nil
}
