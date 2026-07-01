package summary

import (
	"encoding/json"
	"log/slog"
	"os"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type Meta struct {
	LastMessageTime string `json:"last_message_time"`
}

func GetMeta(sessionID string) Meta {
	p := filesystem.SummaryMetaPath(sessionID)
	if !go_pkg_filesystem_reader.Exists(p) {
		return Meta{}
	}
	meta, err := go_pkg_filesystem.ReadJSON[Meta](p)
	if err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem ReadJSON",
			slog.String("path", p),
			slog.String("error", err.Error()))
		return Meta{}
	}
	return meta
}

func SaveMeta(sessionID string, lastTime string) {
	meta := Meta{LastMessageTime: lastTime}
	if err := go_pkg_filesystem.WriteJSON(filesystem.SummaryMetaPath(sessionID), meta, false); err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem WriteJSON",
			slog.String("path", filesystem.SummaryMetaPath(sessionID)),
			slog.String("error", err.Error()))
	}
}

func Get(sessionID string) ([]byte, map[string]any) {
	text, err := go_pkg_filesystem.ReadText(filesystem.SummaryPath(sessionID))
	if err != nil {
		return nil, nil
	}
	raw := []byte(text)

	var dic map[string]any
	if err := json.Unmarshal(raw, &dic); err != nil {
		slog.Warn("json Unmarshal",
			slog.String("path", filesystem.SummaryPath(sessionID)),
			slog.String("error", err.Error()))
		return raw, nil
	}
	return raw, dic
}

func Save(sessionID string, data any) {
	if err := go_pkg_filesystem.WriteJSON(filesystem.SummaryPath(sessionID), data, false); err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem WriteJSON",
			slog.String("path", filesystem.SummaryPath(sessionID)),
			slog.String("error", err.Error()))
	}
}

func Ensure(sessionID string) ([]byte, map[string]any) {
	raw, summary := Get(sessionID)
	if raw != nil {
		return raw, summary
	}

	empty := map[string]any{}
	Save(sessionID, empty)
	raw, summary = Get(sessionID)
	if raw != nil {
		return raw, summary
	}

	return []byte("{}"), empty
}

func Pending() []string {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return nil
	}

	var list []string
	for _, dir := range dirs {
		sid := dir.Name
		historyPath := filesystem.HistoryPath(sid)
		hInfo, err := os.Stat(historyPath)
		if err != nil {
			continue
		}

		summaryPath := filesystem.SummaryPath(sid)
		info, err := os.Stat(summaryPath)
		if err != nil || hInfo.ModTime().After(info.ModTime()) {
			list = append(list, sid)
		}
	}
	return list
}
