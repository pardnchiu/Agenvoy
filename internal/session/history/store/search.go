package store

import (
	"fmt"
	"strings"
	"time"
)

type Result struct {
	Timestamp int64
	Role      string
	Content   string
}

var searchTimeRanges = map[string]time.Duration{
	"1d": 24 * time.Hour,
	"7d": 7 * 24 * time.Hour,
	"1m": 30 * 24 * time.Hour,
	"1y": 365 * 24 * time.Hour,
}

func Search(sessionID, keyword, timeRange string, limit int) ([]Result, error) {
	if conn == nil {
		return nil, nil
	}

	jsonStart := GetStartAt(sessionID)

	escaped := strings.ReplaceAll(keyword, `"`, `""`)
	ftsQuery := fmt.Sprintf(`"%s"`, escaped)

	var after int64
	if d, ok := searchTimeRanges[timeRange]; ok {
		after = time.Now().Add(-d).UnixNano()
	}

	before := max(jsonStart, 0)

	rows, err := conn.Query(`
	SELECT m.send_at, m.role, m.content
	FROM messages m
	WHERE m.session_id = ?
	AND m.id IN (SELECT rowid FROM messages_fts5 WHERE messages_fts5 MATCH ?)
	AND m.send_at >= ?
	AND m.send_at < ?
	ORDER BY m.send_at DESC
	LIMIT ?
	`, sessionID, ftsQuery, after, before, limit)
	if err != nil {
		return nil, fmt.Errorf("sql.DB Query [SELECT messages]: %w", err)
	}
	defer rows.Close()

	var list []Result
	for rows.Next() {
		var result Result
		if err := rows.Scan(&result.Timestamp, &result.Role, &result.Content); err != nil {
			continue
		}
		list = append(list, result)
	}
	return list, rows.Err()
}
