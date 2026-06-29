package fetchPage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
)

func isSkipped(href string) (bool, int, string) {
	db := torii.DB(torii.DBToolCache)
	for _, prefix := range []string{"skip4xx:", "skip5xx:", "skipEmpty:"} {
		entry, ok := db.Get(skipKey(prefix, href))
		if !ok {
			continue
		}
		status, title := parseSkipValue(entry.Value())
		return true, status, title
	}
	return false, 0, ""
}

func skipKey(prefix, href string) string {
	hash := sha256.Sum256([]byte(href))
	return prefix + hex.EncodeToString(hash[:])
}

func addToSkippedMap(href string, status int, title string) {
	db := torii.DB(torii.DBToolCache)
	val := fmt.Sprintf("%d|%s", status, strings.TrimSpace(title))
	if status >= 500 {
		if err := db.Set(skipKey("skip5xx:", href), val, torii.SetDefault, torii.TTL(int64(skippedExpired.Seconds()))); err != nil {
			slog.Warn("store.DB.Set",
				slog.String("error", err.Error()))
		}
		return
	}
	if status == 0 {
		if err := db.Set(skipKey("skipEmpty:", href), val, torii.SetDefault, torii.TTL(int64(emptySkipExpired.Seconds()))); err != nil {
			slog.Warn("store.DB.Set",
				slog.String("error", err.Error()))
		}
		return
	}
	if err := db.Set(skipKey("skip4xx:", href), val, torii.SetDefault, nil); err != nil {
		slog.Warn("store.DB.Set",
			slog.String("error", err.Error()))
	}
}

func parseSkipValue(raw string) (int, string) {
	idx := strings.Index(raw, "|")
	if idx < 0 {
		return 0, ""
	}
	status, _ := strconv.Atoi(raw[:idx])
	return status, raw[idx+1:]
}
