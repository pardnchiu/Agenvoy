package errorMemory

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
)

func Search(tool, keyword string, limit int) string {
	limit = clampLimit(limit)

	if tool == "" && keyword == "" {
		return "keyword is required when tool is not specified"
	} else if tool == "" && keyword != "" {
		records := scanAll(keyword, limit)
		if len(records) == 0 {
			return "NONE"
		}
		return format(records, limit)
	}

	pattern := tool + ":*"
	msg := getMessage(keyword)
	if msg == "unknown" {
		return "NONE"
	}

	records := scanWithFilter(pattern, func(rec Record) bool {
		if slices.Contains(rec.Keywords, "error_type:"+msg) {
			return true
		}
		return false
	}, limit)
	if len(records) > 0 {
		return format(records, limit)
	}
	return "NONE"
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

func scanAll(keyword string, cap int) []Record {
	lower := strings.ToLower(keyword)
	return scanWithFilter("*", func(rec Record) bool {
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
	}, cap)
}

func scanWithFilter(pattern string, match func(Record) bool, cap int) []Record {
	db := torii.DB(torii.DBErrorMemory)
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
		out = append(out, rec)
		if len(out) >= cap {
			break
		}
	}
	return out
}
