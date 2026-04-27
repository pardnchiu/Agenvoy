package toolError

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"

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
	recordBytes, err := json.Marshal(record)
	if err != nil {
		return hash
	}

	dir := filepath.Join(filesystem.SessionsDir, sessionID, "tool_errors")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return hash
	}
	if err := go_utils_filesystem.WriteFile(filepath.Join(dir, hash+".json"), string(recordBytes), 0644); err != nil {
		slog.Warn("failed to save tool error",
			slog.String("error", err.Error()))
	}
	return hash
}

func Get(sessionID, hash string) string {
	path := filepath.Join(filesystem.SessionsDir, sessionID, "tool_errors", hash+".json")
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("failed to get tool error",
			slog.String("error", err.Error()))
		return ""
	}
	return string(fileBytes)
}
