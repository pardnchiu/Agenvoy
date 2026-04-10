package file

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
)

type ErrorMemory struct {
	ID        string   `json:"id"`
	Timestamp int64    `json:"timestamp"`
	ToolName  string   `json:"tool_name"`
	Keywords  []string `json:"keywords"`
	Symptom   string   `json:"symptom"`
	Cause     string   `json:"cause,omitempty"`
	Action    string   `json:"action"`
	Outcome   string   `json:"outcome,omitempty"`
}

func classifyErrorText(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case lower == "":
		return "unknown"
	case strings.Contains(lower, "not exist:"),
		strings.Contains(lower, "tool not found"),
		strings.Contains(lower, "unsupported tool"):
		return "tool_not_exist"
	case strings.Contains(lower, "required"):
		return "required_param"
	case strings.Contains(lower, "invalid"):
		return "invalid_param"
	case strings.Contains(lower, "timeout"),
		strings.Contains(lower, "deadline exceeded"):
		return "timeout"
	case strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "forbidden"),
		strings.Contains(lower, "permission denied"),
		strings.Contains(lower, "access denied"):
		return "permission"
	case strings.Contains(lower, "no result"),
		strings.Contains(lower, "no data"),
		strings.Contains(lower, "not found"),
		strings.Contains(lower, "empty result"):
		return "no_result"
	default:
		return "other"
	}
}

func normalizeKeywords(keywords []string) []string {
	if len(keywords) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(keywords))
	result := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		text := strings.TrimSpace(strings.ToLower(keyword))
		if text == "" {
			continue
		}
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		result = append(result, text)
	}
	return result
}

func SaveErrorMemory(sessionID string, record ErrorMemory) (string, error) {
	record.Keywords = normalizeKeywords(record.Keywords)
	errType := classifyErrorText(record.Symptom + "\n" + record.Cause)
	if errType != "unknown" {
		record.Keywords = normalizeKeywords(append(record.Keywords, "error_type:"+errType))
	}

	now := time.Now()
	h := sha256.Sum256([]byte(record.ToolName + strconv.FormatInt(now.UnixNano(), 10)))
	record.ID = hex.EncodeToString(h[:])
	record.Timestamp = now.Unix()

	raw, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}

	key := fmt.Sprintf("%s:%d", record.ToolName, now.UnixNano())
	if err := store.DB(store.DBErrorMemory).Set(key, string(raw), store.SetDefault, nil); err != nil {
		return "", fmt.Errorf("store.Set: %w", err)
	}

	return fmt.Sprintf("Remember the Error: %s", record.ID), nil
}

func SearchErrors(keyword string, limit int) (string, error) {
	if keyword == "" {
		return "", fmt.Errorf("keyword is required")
	}
	limit = clampLimit(limit)

	matched := scanErrorMemory("*", keyword, limit*4)
	if len(matched) == 0 {
		return "NONE", nil
	}
	return formatRecords(matched, limit), nil
}

func SearchErrorMemory(tool, keyword string, limit int) string {
	limit = clampLimit(limit)
	pattern := tool + ":*"

	if errType := classifyErrorText(keyword); errType != "unknown" {
		records := scanErrorMemoryFiltered(pattern, func(rec ErrorMemory) bool {
			for _, k := range rec.Keywords {
				if k == "error_type:"+errType {
					return true
				}
			}
			return false
		}, limit)
		if len(records) > 0 {
			return formatRecords(records, limit)
		}
	}

	if keyword != "" {
		records := scanErrorMemory(pattern, keyword, limit)
		if len(records) > 0 {
			return formatRecords(records, limit)
		}
	}

	records := scanErrorMemoryFiltered(pattern, func(ErrorMemory) bool { return true }, limit)
	if len(records) == 0 {
		return ""
	}
	return formatRecords(records, limit)
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

func scanErrorMemory(pattern, keyword string, cap int) []ErrorMemory {
	lower := strings.ToLower(keyword)
	return scanErrorMemoryFiltered(pattern, func(rec ErrorMemory) bool {
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

func scanErrorMemoryFiltered(pattern string, match func(ErrorMemory) bool, cap int) []ErrorMemory {
	db := store.DB(store.DBErrorMemory)
	keys := db.Keys(pattern)
	if len(keys) == 0 {
		return nil
	}

	out := make([]ErrorMemory, 0, cap)
	for i := len(keys) - 1; i >= 0; i-- {
		entry, ok := db.Get(keys[i])
		if !ok {
			continue
		}
		var rec ErrorMemory
		if err := json.Unmarshal([]byte(entry.Value), &rec); err != nil {
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

func formatRecords(records []ErrorMemory, limit int) string {
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Timestamp > records[j].Timestamp
	})
	if len(records) > limit {
		records = records[:limit]
	}
	out, err := json.Marshal(records)
	if err != nil {
		return ""
	}
	return string(out)
}
