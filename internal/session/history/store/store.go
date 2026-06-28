package store

import (
	_ "embed"
	"fmt"
	"sync"

	go_sqlkit "github.com/pardnchiu/go-sqlkit"
	go_sqlkit_core "github.com/pardnchiu/go-sqlkit/core"
)

//go:embed migrate.sql
var migrateSQL string

var (
	once sync.Once
	conn *go_sqlkit_core.Connector
)

func New(dbPath string) error {
	var initErr error
	once.Do(func() {
		c, err := go_sqlkit.New(go_sqlkit_core.Config{Target: go_sqlkit_core.SQLite, Path: dbPath})
		if err != nil {
			initErr = fmt.Errorf("github.com/pardnchiu/go-sqlkit New: %w", err)
			return
		}
		if _, err := c.Exec(migrateSQL); err != nil {
			c.Close()
			initErr = fmt.Errorf("sql.DB Exec [migrate]: %w", err)
			return
		}
		conn = c
	})
	return initErr
}

func Close() {
	if conn == nil {
		return
	}
	conn.Close()
}

func IsReady() bool {
	return conn != nil
}

func IsExist(sessionID string) bool {
	if conn == nil {
		return false
	}

	var exists bool
	conn.Read.QueryRow(`
	SELECT EXISTS(SELECT 1 FROM messages WHERE session_id = ?)
	`, sessionID).Scan(&exists)
	return exists
}

func SetStartAt(sessionID string, timestamp int64) error {
	if conn == nil {
		return nil
	}

	_, err := conn.Exec(`
	INSERT INTO session_meta (session_id, start_at)
	VALUES (?, ?)
	ON CONFLICT(session_id)
	DO UPDATE SET start_at = excluded.start_at
	`, sessionID, timestamp)
	return err
}

func GetStartAt(sessionID string) int64 {
	if conn == nil {
		return 0
	}

	var ts int64
	conn.Read.QueryRow(`
	SELECT start_at
	FROM session_meta
	WHERE session_id = ?
	`, sessionID).Scan(&ts)
	return ts
}
