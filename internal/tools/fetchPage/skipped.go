package fetchPage

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"

	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
)

func skipKey(href string) string {
	hash := sha256.Sum256([]byte(href))
	return "skip:" + hex.EncodeToString(hash[:])
}

func isSkipped(href string) bool {
	key := skipKey(href)
	if _, ok := store.DB(store.DBFetchSkip4xx).Get(key); ok {
		return true
	}
	if _, ok := store.DB(store.DBFetchSkip5xx).Get(key); ok {
		return true
	}
	return false
}

func addToSkippedMap(href string, status int) {
	key := skipKey(href)
	if status >= 500 {
		if err := store.DB(store.DBFetchSkip5xx).Set(key, "1", store.SetDefault, store.TTL(int64(skippedExpired.Seconds()))); err != nil {
			slog.Warn("store.DB.Set",
				slog.String("error", err.Error()))
		}
		return
	}
	if err := store.DB(store.DBFetchSkip4xx).Set(key, "1", store.SetDefault, nil); err != nil {
		slog.Warn("store.DB.Set",
			slog.String("error", err.Error()))
	}
}
