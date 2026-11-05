package interactive

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
)

const (
	onlineTTL      = 60
	onlineInterval = 55 * time.Second
)

func onlineKey(sessionID, taskHash string) string {
	return "action:" + sessionID + ":" + taskHash
}

func markOnline(sessionID, taskHash string) {
	db := torii.DB(torii.DBOnline)
	if err := db.Set(context.Background(), onlineKey(sessionID, taskHash), "1", torii.TTL(onlineTTL)); err != nil {
		slog.Debug("markOnline",
			slog.String("session", sessionID),
			slog.String("task", taskHash),
			slog.String("error", err.Error()))
	}
}

func ClearOnline() int {
	db := torii.DB(torii.DBOnline)
	keys := db.Keys(context.Background(), "action:*")
	if len(keys) == 0 {
		return 0
	}
	return db.Del(context.Background(), keys...)
}

func ActiveCount(sessionID string) int {
	if sessionID == "" {
		return 0
	}
	return len(torii.DB(torii.DBOnline).Keys(context.Background(), onlineKey(sessionID, "*")))
}

func ActiveCounts() map[string]int {
	keys := torii.DB(torii.DBOnline).Keys(context.Background(), "action:*")
	dic := make(map[string]int, len(keys))
	for _, key := range keys {
		rest, ok := strings.CutPrefix(key, "action:")
		if !ok {
			continue
		}
		sessionID, _, ok := strings.Cut(rest, ":")
		if !ok || sessionID == "" {
			continue
		}
		dic[sessionID]++
	}
	return dic
}

var claimMu sync.Mutex

func ClaimOnline(sessionID, taskHash string) bool {
	claimMu.Lock()
	defer claimMu.Unlock()
	if IsOnline(sessionID, taskHash) {
		return false
	}
	markOnline(sessionID, taskHash)
	return true
}

func ReleaseOnline(sessionID, taskHash string) {
	torii.DB(torii.DBOnline).Del(context.Background(), onlineKey(sessionID, taskHash))
}

func IsOnline(sessionID, taskHash string) bool {
	_, ok := torii.DB(torii.DBOnline).Get(context.Background(), onlineKey(sessionID, taskHash))
	return ok
}

func ListResumablePending(sessionID string) []string {
	hashes := listPendingTasks(sessionID)
	list := make([]string, 0, len(hashes))
	for _, one := range hashes {
		if IsOnline(sessionID, one) {
			continue
		}
		list = append(list, one)
	}
	return list
}

func ListOnlinePending(sessionID string) []string {
	hashes := listPendingTasks(sessionID)
	list := make([]string, 0, len(hashes))
	for _, one := range hashes {
		if IsOnline(sessionID, one) {
			list = append(list, one)
		}
	}
	return list
}

func KeepOnline(sessionID, taskHash string) func() {
	markOnline(sessionID, taskHash)

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(onlineInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				markOnline(sessionID, taskHash)
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			ReleaseOnline(sessionID, taskHash)
		})
	}
}
