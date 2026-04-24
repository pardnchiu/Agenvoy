package errorMemory

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"sort"
	"strings"

	toriidb "github.com/pardnchiu/ToriiDB/core/store"
	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
)

func Search(ctx context.Context, tool, keyword string, limit int) string {
	limit = clampLimit(limit)

	if tool == "" && keyword == "" {
		return "keyword is required when tool is not specified"
	}

	db := torii.DB(torii.DBErrorMemory)

	pattern := "*"
	if tool != "" {
		pattern = tool + ":*"
	}

	if keyword != "" {
		if records := vectorSearch(ctx, db, pattern, keyword, limit); len(records) > 0 {
			return format(records, limit)
		}
	}

	records := keywordScan(db, tool, keyword, limit)
	if len(records) == 0 {
		return "NONE"
	}
	return format(records, limit)
}

func vectorSearch(ctx context.Context, db *toriidb.Session, pattern, keyword string, limit int) []Record {
	keys, err := db.VSearch(ctx, keyword, pattern, limit)
	if err != nil || len(keys) == 0 {
		return nil
	}

	out := make([]Record, 0, len(keys))
	for _, key := range keys {
		entry, ok := db.Get(key)
		if !ok {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(entry.Value()), &rec); err != nil {
			continue
		}
		if err := db.Expire(key, ttlSeconds); err != nil {
			slog.Warn("errorMemory.Expire",
				slog.String("key", key),
				slog.String("error", err.Error()))
		}
		out = append(out, rec)
	}
	return out
}

func keywordScan(db *toriidb.Session, tool, keyword string, limit int) []Record {
	if tool != "" {
		msg := getMessage(keyword)
		if msg == "unknown" {
			return nil
		}
		return scanWithFilter(db, tool+":*", func(rec Record) bool {
			return slices.Contains(rec.Keywords, "error_type:"+msg)
		}, limit)
	}

	lower := strings.ToLower(keyword)
	return scanWithFilter(db, "*", func(rec Record) bool {
		if lower == "" {
			return true
		}
		if strings.Contains(strings.ToLower(rec.ToolName), lower) ||
			strings.Contains(strings.ToLower(rec.Symptom), lower) ||
			strings.Contains(strings.ToLower(rec.Cause), lower) {
			return true
		}
		for _, kw := range rec.Keywords {
			text := strings.ToLower(kw)
			if strings.Contains(text, lower) || strings.Contains(lower, text) {
				return true
			}
		}
		return false
	}, limit)
}

func scanWithFilter(db *toriidb.Session, pattern string, match func(Record) bool, cap int) []Record {
	keys := db.Keys(pattern)
	if len(keys) == 0 {
		return nil
	}

	out := make([]Record, 0, cap)
	for i := len(keys) - 1; i >= 0; i-- {
		entry, ok := db.Get(keys[i])
		if !ok {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(entry.Value()), &rec); err != nil {
			continue
		}
		if !match(rec) {
			continue
		}
		if err := db.Expire(keys[i], ttlSeconds); err != nil {
			slog.Warn("errorMemory.Expire",
				slog.String("key", keys[i]),
				slog.String("error", err.Error()))
		}
		out = append(out, rec)
		if len(out) >= cap {
			break
		}
	}
	return out
}

func format(records []Record, limit int) string {
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Timestamp > records[j].Timestamp
	})
	if len(records) > limit {
		records = records[:limit]
	}

	recordBytes, err := json.Marshal(records)
	if err != nil {
		return ""
	}
	return string(recordBytes)
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 4
	}
	if limit > 16 {
		return 16
	}
	return limit
}
