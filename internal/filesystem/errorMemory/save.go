package errorMemory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
)

type Record struct {
	ID        string   `json:"id"`
	Timestamp int64    `json:"timestamp"`
	ToolName  string   `json:"tool_name"`
	Keywords  []string `json:"keywords"`
	Symptom   string   `json:"symptom"`
	Cause     string   `json:"cause,omitempty"`
	Action    string   `json:"action"`
	Outcome   string   `json:"outcome,omitempty"`
}

func Save(sessionID string, record Record) (string, error) {
	record.Keywords = getKeywords(record.Keywords)
	errType := getMessage(record.Symptom + "\n" + record.Cause)
	if errType != "unknown" {
		record.Keywords = getKeywords(append(record.Keywords, "error_type:"+errType))
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
	if err := torii.DB(torii.DBErrorMemory).Set(key, string(raw), torii.SetDefault, nil); err != nil {
		return "", fmt.Errorf("store.Set: %w", err)
	}

	return fmt.Sprintf("Remember the Error: %s", record.ID), nil
}

func getKeywords(keywords []string) []string {
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

func getMessage(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	switch {
	case text == "":
		return "unknown"
	case strings.Contains(text, "not exist:"),
		strings.Contains(text, "tool not found"),
		strings.Contains(text, "unsupported tool"):
		return "tool_not_exist"
	case strings.Contains(text, "required"):
		return "required_param"
	case strings.Contains(text, "invalid"):
		return "invalid_param"
	case strings.Contains(text, "timeout"),
		strings.Contains(text, "deadline exceeded"):
		return "timeout"
	case strings.Contains(text, "unauthorized"),
		strings.Contains(text, "forbidden"),
		strings.Contains(text, "permission denied"),
		strings.Contains(text, "access denied"):
		return "permission"
	case strings.Contains(text, "no result"),
		strings.Contains(text, "no data"),
		strings.Contains(text, "not found"),
		strings.Contains(text, "empty result"):
		return "no_result"
	default:
		return "other"
	}
}
