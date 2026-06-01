package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
)

const ttlSeconds int64 = 90 * 24 * 3600

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
	db := torii.DB(torii.DBErrorMemory)
	value := string(raw)
	expireAt := torii.TTL(ttlSeconds)

	if err := db.SetVector(context.Background(), key, value, torii.SetDefault, expireAt); err != nil {
		if err = db.Set(key, value, torii.SetDefault, expireAt); err != nil {
			return "", fmt.Errorf("store.Set: %w", err)
		}
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
		str := strings.TrimSpace(strings.ToLower(keyword))
		if str == "" {
			continue
		}
		if _, ok := seen[str]; ok {
			continue
		}
		seen[str] = struct{}{}
		result = append(result, str)
	}
	return result
}

func getMessage(str string) string {
	str = strings.ToLower(strings.TrimSpace(str))
	switch {
	case str == "":
		return "unknown"
	case strings.Contains(str, "not exist:"),
		strings.Contains(str, "tool not found"),
		strings.Contains(str, "unsupported tool"):
		return "tool_not_exist"
	case strings.Contains(str, "required"):
		return "required_param"
	case strings.Contains(str, "invalid"):
		return "invalid_param"
	case strings.Contains(str, "timeout"),
		strings.Contains(str, "deadline exceeded"):
		return "timeout"
	case strings.Contains(str, "unauthorized"),
		strings.Contains(str, "forbidden"),
		strings.Contains(str, "permission denied"),
		strings.Contains(str, "access denied"):
		return "permission"
	case strings.Contains(str, "no result"),
		strings.Contains(str, "no data"),
		strings.Contains(str, "not found"),
		strings.Contains(str, "empty result"):
		return "no_result"
	default:
		return "other"
	}
}
