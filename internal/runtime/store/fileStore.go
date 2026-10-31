package historyStore

import (
	"context"
	"time"

	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const keepPerPath = 32

func insert(ctx context.Context, c Change, meta Meta) error {
	var content any
	if len(c.content) > 0 {
		content = c.content
	}
	var trashPath any
	if c.trashPath != "" {
		trashPath = c.trashPath
	}
	truncated := 0
	if c.truncated {
		truncated = 1
	}

	if _, err := conn.ExecContext(ctx, `
	INSERT INTO file_history
	(dir, name, action, content, hash, size, truncated, trash_path, session_id, task_id, tool, changed_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(dir, name, task_id, session_id) WHERE task_id != '' DO NOTHING
	`,
		c.dir, c.name, c.action, content, c.hash, c.size, truncated, trashPath,
		meta.SessionID, meta.TaskID, meta.Tool, time.Now().Truncate(time.Second).UnixNano(),
	); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "sql.DB ExecContext", "[INSERT file_history]", err)
	}

	return nil
}

func RenameFileTask(ctx context.Context, sessionID, oldID, newID string) error {
	if conn == nil || oldID == "" {
		return nil
	}

	if _, err := conn.ExecContext(ctx, `
	UPDATE file_history SET task_id = ?
	WHERE session_id = ? AND task_id = ?`, newID, sessionID, oldID); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "sql.DB ExecContext", "[UPDATE file_history]", err)
	}
	renameTaskOrigin(oldID, newID)
	return nil
}

func PruneFile(ctx context.Context) error {
	if conn == nil {
		return nil
	}

	if _, err := conn.ExecContext(ctx, `
	DELETE FROM file_history
	WHERE id IN (
		SELECT id FROM (
			SELECT id,
				ROW_NUMBER() OVER (PARTITION BY dir, name ORDER BY changed_at DESC, id DESC) AS seq
			FROM file_history
		)
		WHERE seq > ?
	)`, keepPerPath); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "sql.DB ExecContext", "[DELETE file_history]", err)
	}
	return nil
}

func latestModifyHash(ctx context.Context, dir, name string) string {
	var action, hash string
	conn.Read.QueryRowContext(ctx, `
	SELECT action, hash
	FROM file_history
	WHERE dir = ? AND name = ?
	ORDER BY changed_at DESC, id DESC
	LIMIT 1
	`, dir, name).Scan(&action, &hash)

	if action != ActionModify {
		return ""
	}
	return hash
}
