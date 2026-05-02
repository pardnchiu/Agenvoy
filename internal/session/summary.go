package session

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func SummaryPath(sessionID string) string {
	return filepath.Join(filesystem.SessionsDir, sessionID, "summary.json")
}

func GetSummary(sessionID string) ([]byte, map[string]any) {
	text, err := go_pkg_filesystem.ReadText(SummaryPath(sessionID))
	if err != nil {
		return nil, nil
	}
	bytes := []byte(text)

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
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return nil
	}

	var result []string
	for _, dir := range dirs {
		sid := dir.Name
		historyPath := filepath.Join(filesystem.SessionsDir, sid, "history.json")
		// * os.Stat retained: FileInfo.ModTime() needed to compare history vs summary recency
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
	if err := go_pkg_filesystem.WriteJSON(SummaryPath(sessionID), data, false); err != nil {
		slog.Warn("WriteJSON",
			slog.String("error", err.Error()))
	}
}
