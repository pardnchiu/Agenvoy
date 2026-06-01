CREATE TABLE IF NOT EXISTS messages (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id   TEXT    NOT NULL,
    send_at      INTEGER NOT NULL DEFAULT 0,
    role         TEXT    NOT NULL,
    content      TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_session_send_at
    ON messages(session_id, send_at);

CREATE TABLE IF NOT EXISTS session_meta (
    session_id TEXT PRIMARY KEY,
    start_at   INTEGER NOT NULL DEFAULT 0
);

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts5 USING fts5(
    role, content,
    content=messages, content_rowid=id
);

CREATE TRIGGER IF NOT EXISTS trigger_messages_after_insert AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts5(rowid, role, content)
    VALUES (new.id, new.role, new.content);
END;

CREATE TRIGGER IF NOT EXISTS trigger_messages_after_delete AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts5(messages_fts5, rowid, role, content)
    VALUES ('delete', old.id, old.role, old.content);
END;
