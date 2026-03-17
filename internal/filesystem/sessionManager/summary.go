package sessionManager

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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

func GetSummaryPrompt(sessionID string) string {
	bytes, _ := GetSummary(sessionID)
	summary := strings.NewReplacer(
		"{{.Summary}}", string(bytes),
	).Replace(strings.TrimSpace(configs.SummaryPrompt))
	return summary
}

func SaveSummary(sessionID string, data any) {
	if bytes, err := json.Marshal(data); err == nil {
		if err := filesystem.WriteFile(SummaryPath(sessionID), string(bytes), 0644); err != nil {
			slog.Warn("WriteFile",
				slog.String("error", err.Error()))
		}
	} else {
		slog.Warn("json.Marshal",
			slog.String("error", err.Error()))
	}
}
