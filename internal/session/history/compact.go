package history

import (
	"encoding/json"
	"log/slog"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	historyStore "github.com/pardnchiu/agenvoy/internal/session/history/store"
)

func compact(sessionID, historyPath string, messages []agentTypes.Message, currentBytes int) {
	if len(messages) < 4 {
		return
	}

	targetBtyes := int(float64(filesystem.MaxHistoryBytes) * 0.8)
	needRemove := currentBytes - targetBtyes
	if needRemove <= 0 {
		return
	}

	size := 0
	idx := 0
	for i, message := range messages {
		raw, err := json.Marshal(message)
		if err != nil {
			continue
		}
		size += len(raw) + 1
		if size >= needRemove {
			idx = i + 1
			break
		}
	}

	for idx < len(messages) && messages[idx-1].Role != "assistant" {
		idx++
	}
	if idx <= 0 || idx >= len(messages) {
		return
	}

	remaining := messages[idx:]
	startAt := getStartAt(remaining)
	if startAt > 0 {
		if err := historyStore.SetStartAt(sessionID, startAt); err != nil {
			slog.Warn("historyStore SetStart",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}
		clean(sessionID, startAt)
	}

	raw, err := json.Marshal(remaining)
	if err != nil {
		slog.Warn("json marshal",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
		return
	}
	if err := go_pkg_filesystem.WriteFile(historyPath, string(raw), 0644); err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem WriteFile",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
		return
	}
}

func clean(sessionID string, before int64) {
	db := torii.DB(torii.DBSessionHist)
	keys := db.Keys(sessionID + ":*")
	if len(keys) == 0 {
		return
	}
	var toDelete []string
	for _, key := range keys {
		ts, ok := getTimestamp(key)
		if !ok {
			continue
		}
		if ts < before {
			toDelete = append(toDelete, key)
		}
	}
	if len(toDelete) > 0 {
		db.Del(toDelete...)
	}
}

func getTimestamp(key string) (int64, bool) {
	idx := len(key) - 1
	for idx >= 0 && key[idx] != ':' {
		idx--
	}
	if idx < 0 {
		return 0, false
	}
	var ts int64
	for _, c := range key[idx+1:] {
		if c < '0' || c > '9' {
			return 0, false
		}
		ts = ts*10 + int64(c-'0')
	}
	return ts, true
}

func getStartAt(messages []agentTypes.Message) int64 {
	for _, msg := range messages {
		content := historyStore.ExtractContent(msg.Content)
		ts := historyStore.ExtractTimestamp(content)
		if ts > 0 {
			return ts
		}
	}
	return 0
}
