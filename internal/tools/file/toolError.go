package file

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type ToolError struct {
	Hash      string `json:"hash"`
	Timestamp int64  `json:"timestamp"`
	ToolName  string `json:"tool_name"`
	Args      string `json:"args"`
	Error     string `json:"error"`
}

func SaveToolError(sessionID, toolName, args, errMsg string) string {
	raw := toolName + "|" + args + "|" + errMsg
	h := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(h[:])[:8]

	record := ToolError{
		Hash:      hash,
		Timestamp: time.Now().Unix(),
		ToolName:  toolName,
		Args:      args,
		Error:     errMsg,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return hash
	}

	dir := filepath.Join(filesystem.SessionsDir, sessionID, "tool_errors")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return hash
	}
	filesystem.WriteFile(filepath.Join(dir, hash+".json"), string(data), 0644)
	return hash
}

func GetToolError(sessionID, hash string) string {
	path := filepath.Join(filesystem.SessionsDir, sessionID, "tool_errors", hash+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
