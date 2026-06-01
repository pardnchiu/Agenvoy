package store

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

var timestampRegex = regexp.MustCompile(`當前時間:\s*(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)

func Write(sessionID string, messages []agentTypes.Message) error {
	if conn == nil || len(messages) == 0 {
		return nil
	}

	tx, err := conn.Write.Raw().Begin()
	if err != nil {
		return fmt.Errorf("sql.Tx Begin: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
	INSERT INTO messages (session_id, send_at, role, content)
	VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("sql.Tx Prepare [INSERT messages]: %w", err)
	}
	defer stmt.Close()

	var lastTS int64
	for _, msg := range messages {
		content := ExtractContent(msg.Content)
		if strings.TrimSpace(content) == "" {
			continue
		}

		ts := ExtractTimestamp(content)
		if ts == 0 && lastTS > 0 {
			ts = lastTS + 1
		}
		if ts > lastTS {
			lastTS = ts
		}

		if _, err := stmt.Exec(sessionID, ts, msg.Role, content); err != nil {
			return fmt.Errorf("sql.Stmt Exec: %w", err)
		}
	}

	return tx.Commit()
}

func ExtractContent(content any) string {
	switch val := content.(type) {
	case string:
		return val

	case []any:
		var parts []string
		for _, item := range val {
			dic, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, _ := dic["type"].(string); text == "text" {
				if text, _ := dic["text"].(string); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")

	default:
		if content == nil {
			return ""
		}
		return fmt.Sprint(content)
	}
}

func ExtractTimestamp(content string) int64 {
	match := timestampRegex.FindStringSubmatch(content)
	if len(match) < 2 {
		return 0
	}

	t, err := time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
	if err != nil {
		return 0
	}
	return t.UnixNano()
}

func Clear(sessionID string) error {
	if conn == nil {
		return nil
	}

	if _, err := conn.Write.Raw().Exec(`
	DELETE FROM messages
	WHERE session_id = ?
	`, sessionID); err != nil {
		return fmt.Errorf("sql.DB Exec [DELETE messages]: %w", err)
	}

	if _, err := conn.Write.Raw().Exec(`
	DELETE FROM session_meta
	WHERE session_id = ?
	`, sessionID); err != nil {
		return fmt.Errorf("sql.DB Exec [DELETE messages_meta]: %w", err)
	}
	return nil
}
