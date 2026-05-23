package filesystem

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

type ToolError struct {
	Hash      string `json:"hash"`
	Timestamp int64  `json:"timestamp"`
	ToolName  string `json:"tool_name"`
	Args      string `json:"args"`
	Error     string `json:"error"`
}

func GetError(sessionID, hash string) string {
	path := ErrorPath(sessionID, hash)
	result, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem ReadText",
			slog.String("session", sessionID),
			slog.String("path", path),
			slog.String("error", err.Error()))
		return ""
	}
	return result
}

func SaveError(sessionID, toolName, args, errMsg string) string {
	hashFull := sha256.Sum256([]byte(toolName + "|" + args + "|" + errMsg))
	hash := hex.EncodeToString(hashFull[:])[:8]
	record := ToolError{
		Hash:      hash,
		Timestamp: time.Now().Unix(),
		ToolName:  toolName,
		Args:      args,
		Error:     errMsg,
	}
	dir := ErrorDir(sessionID)
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem CheckDir",
			slog.String("session", sessionID),
			slog.String("dir", dir),
			slog.String("error", err.Error()))
		return hash
	}
	path := ErrorPath(sessionID, hash)
	if err := go_pkg_filesystem.WriteJSON(path, record, false); err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem WriteJSON",
			slog.String("session", sessionID),
			slog.String("path", path),
			slog.String("error", err.Error()))
	}
	return hash
}
