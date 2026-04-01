package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

var (
	timeRegex = regexp.MustCompile(`(---\n)*當前時間: (\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)
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
		Description: "Search the current session's conversation history by keyword. Returns full message entries (role and content) that contain the keyword. Supports time range filtering.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Keyword to search for (case-insensitive, literal string match)",
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
				TimeRange string `json:"time_range"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return searchHistory(e.SessionID, params.Keyword, params.TimeRange)
		},
	})
}

func getTimestamp(content string) (int64, string) {
	matches := timeRegex.FindStringSubmatch(content)
	if len(matches) >= 2 {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", matches[2], time.Local); err == nil {
			return t.Unix(), matches[2]
		}
	}
	return 0, content
}

func searchHistory(sessionID, keyword, timeRange string) (string, error) {
	const limit = 8
	if sessionID == "" {
		return "", fmt.Errorf("sessionID is required")
	}
	if keyword == "" {
		return "", fmt.Errorf("keyword is required")
	}

	historyPath := filesystem.HistoryPath(sessionID)
	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "no history found for current session", nil
		}
		return "", fmt.Errorf("failed to read %s: %w", historyPath, err)
	}

	var histories []messageHistory
	if err := json.Unmarshal(data, &histories); err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", historyPath, err)
	}

	var after int64
	if d, ok := historyTimeRanges[timeRange]; ok {
		after = time.Now().Add(-d).Unix()
	}

	lower := strings.ToLower(keyword)
	var matches []messageHistory

	// * skip static history messages
	startIdx := len(histories) - session.MaxHistoryMessages - 1
	if startIdx < 0 {
		return "not much history to search", nil
	}

	for i := startIdx; i >= 0; i-- {
		entry := histories[i]
		ts, body := getTimestamp(entry.Content)
		if after > 0 && ts > 0 && ts < after {
			continue
		}
		if strings.Contains(strings.ToLower(body), lower) {
			matches = append(matches, entry)
			if len(matches) >= limit {
				break
			}
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
