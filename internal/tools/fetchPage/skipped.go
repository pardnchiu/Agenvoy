package fetchPage

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"

	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
)

func skipKey(prefix, href string) string {
	hash := sha256.Sum256([]byte(href))
	return prefix + hex.EncodeToString(hash[:])
}

func isSkipped(href string) bool {
	db := store.DB(store.DBToolCache)
	if _, ok := db.Get(skipKey("skip4xx:", href)); ok {
		return true
	}
	if _, ok := db.Get(skipKey("skip5xx:", href)); ok {
		return true
	}
	return false
}

func addToSkippedMap(href string, status int) {
	db := store.DB(store.DBToolCache)
	if status >= 500 {
		if err := db.Set(skipKey("skip5xx:", href), "1", store.SetDefault, store.TTL(int64(skippedExpired.Seconds()))); err != nil {
			slog.Warn("store.DB.Set",
				slog.String("error", err.Error()))
		}
		return
	}
	if err := db.Set(skipKey("skip4xx:", href), "1", store.SetDefault, nil); err != nil {
		slog.Warn("store.DB.Set",
			slog.String("error", err.Error()))
	}
}
