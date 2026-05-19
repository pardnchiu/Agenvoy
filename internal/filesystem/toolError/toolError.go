package toolError

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"path/filepath"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type ToolError struct {
	Hash      string `json:"hash"`
	Timestamp int64  `json:"timestamp"`
	ToolName  string `json:"tool_name"`
	Args      string `json:"args"`
	Error     string `json:"error"`
}

func Save(sessionID, toolName, args, errMsg string) string {
	hashFull := sha256.Sum256([]byte(toolName + "|" + args + "|" + errMsg))
	hash := hex.EncodeToString(hashFull[:])[:8]
	record := ToolError{
		Hash:      hash,
		Timestamp: time.Now().Unix(),
		ToolName:  toolName,
		Args:      args,
		Error:     errMsg,
	}
	dir := filepath.Join(filesystem.SessionsDir, sessionID, "tool_errors")
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return hash
	}
	if err := go_pkg_filesystem.WriteJSON(filepath.Join(dir, hash+".json"), record, false); err != nil {
		slog.Warn("failed to save tool error",
			slog.String("error", err.Error()))
	}
	return hash
}

func Get(sessionID, hash string) string {
	path := filepath.Join(filesystem.SessionsDir, sessionID, "tool_errors", hash+".json")
	content, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		slog.Warn("failed to get tool error",
			slog.String("error", err.Error()))
		return ""
	}
	return content
}
