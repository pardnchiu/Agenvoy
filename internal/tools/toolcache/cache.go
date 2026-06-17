package toolcache

import (
	"cmp"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
)

const (
	ttlSeconds = 1800
)

type toolHistory struct {
	ToolName  string `json:"tool_name"`
	Args      string `json:"args"`
	Result    string `json:"result"`
	CreatedAt int64  `json:"created_at"`
}

var cacheable = map[string]bool{
	"fetch_page":         true,
	"search_google_news": true,
	"search_web":         true,
}

func IsCacheable(name string) bool {
	return cacheable[name]
}

func keyPrefix(sessionID string) string {
	return "tc:" + sessionID + ":"
}

func Store(sessionID, callID, toolName, args, result string) {
	raw, err := json.Marshal(toolHistory{
		ToolName:  toolName,
		Args:      args,
		Result:    result,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return
	}
	db := torii.DB(torii.DBToolCache)
	if err := db.Set(keyPrefix(sessionID)+callID, string(raw), torii.SetDefault, torii.TTL(ttlSeconds)); err != nil {
		slog.Warn("toolcache Store",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
}

type ListItem struct {
	ID       string
	ToolName string
	Args     string
	Age      time.Duration
}

func List(sessionID string) []ListItem {
	db := torii.DB(torii.DBToolCache)
	prefix := keyPrefix(sessionID)
	keys := db.Keys(prefix + "*")

	now := time.Now()
	var list []ListItem
	for _, k := range keys {
		entry, ok := db.Get(k)
		if !ok {
			continue
		}
		var e toolHistory
		if err := json.Unmarshal([]byte(entry.Value()), &e); err != nil {
			continue
		}
		list = append(list, ListItem{
			ID:       strings.TrimPrefix(k, prefix),
			ToolName: e.ToolName,
			Args:     e.Args,
			Age:      now.Sub(time.Unix(e.CreatedAt, 0)),
		})
	}
	slices.SortFunc(list, func(a, b ListItem) int {
		return cmp.Compare(a.Age, b.Age)
	})
	return list
}

func Get(sessionID, callID string) (string, bool) {
	db := torii.DB(torii.DBToolCache)
	entry, ok := db.Get(keyPrefix(sessionID) + callID)
	if !ok {
		return "", false
	}
	var e toolHistory
	if err := json.Unmarshal([]byte(entry.Value()), &e); err != nil {
		return "", false
	}
	return e.Result, true
}
