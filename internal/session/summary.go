package session

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func SummaryPath(sessionID string) string {
	return filepath.Join(filesystem.SessionsDir, sessionID, "summary.json")
}

func GetSummary(sessionID string) ([]byte, map[string]any) {
	bytes, err := os.ReadFile(SummaryPath(sessionID))
	if err != nil {
		return nil, nil
	}

	var summary map[string]any
	if err := json.Unmarshal(bytes, &summary); err != nil {
		slog.Warn("json.Unmarshal",
			slog.String("error", err.Error()))
		return bytes, nil
	}
	return bytes, summary
}

func EnsureSummary(sessionID string) ([]byte, map[string]any) {
	raw, summary := GetSummary(sessionID)
	if raw != nil {
		return raw, summary
	}

	empty := map[string]any{}
	SaveSummary(sessionID, empty)
	raw, summary = GetSummary(sessionID)
	if raw != nil {
		return raw, summary
	}

	return []byte("{}"), empty
}

func GetSummaryPrompt(sessionID string, cutoff time.Time) string {
	raw, summaryMap := GetSummary(sessionID)
	if raw == nil {
		return ""
	}

	if !cutoff.IsZero() && summaryMap != nil {
		if logs, ok := summaryMap["discussion_log"].([]any); ok {
			filtered := make([]any, 0, len(logs))
			for _, item := range logs {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				t, ok := m["time"].(string)
				if !ok {
					filtered = append(filtered, item)
					continue
				}
				// discussion_log time format: "2006-01-02 15:04"
				if parsed, err := time.ParseInLocation("2006-01-02 15:04", t, time.Local); err == nil {
					if !parsed.Before(cutoff) {
						filtered = append(filtered, item)
					}
				} else {
					filtered = append(filtered, item)
				}
			}
			summaryMap["discussion_log"] = filtered
		}
		if b, err := json.Marshal(summaryMap); err == nil {
			raw = b
		}
	}

	return strings.NewReplacer(
		"{{.Summary}}", string(raw),
	).Replace(strings.TrimSpace(configs.SummaryContext))
}

func IsNeedSummary() []string {
	entries, err := os.ReadDir(filesystem.SessionsDir)
	if err != nil {
		return nil
	}

	var result []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sid := entry.Name()
		historyPath := filepath.Join(filesystem.SessionsDir, sid, "history.json")
		hInfo, err := os.Stat(historyPath)
		if err != nil {
			continue
		}

		summaryPath := SummaryPath(sid)
		sInfo, err := os.Stat(summaryPath)
		if err != nil || hInfo.ModTime().After(sInfo.ModTime()) {
			result = append(result, sid)
		}
	}
	return result
}

func SaveSummary(sessionID string, data any) {
	if bytes, err := json.Marshal(data); err == nil {
		if err := go_utils_filesystem.WriteFile(SummaryPath(sessionID), string(bytes), 0644); err != nil {
			slog.Warn("WriteFile",
				slog.String("error", err.Error()))
		}
	} else {
		slog.Warn("json.Marshal",
			slog.String("error", err.Error()))
	}
}
