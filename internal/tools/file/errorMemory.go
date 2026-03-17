package file

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type ErrorMemory struct {
	ID        string   `json:"id"`
	Timestamp int64    `json:"timestamp"`
	ToolName  string   `json:"tool_name"`
	Keywords  []string `json:"keywords"`
	Symptom   string   `json:"symptom"`
	Cause     string   `json:"cause,omitempty"`
	Action    string   `json:"action"`
	Outcome   string   `json:"outcome,omitempty"`
}

type ErrorMemoryItem struct {
	Count       int   `json:"count"`
	LastUpdated int64 `json:"last_updated"`
}

func SearchErrors(keyword string, limit int) (string, error) {
	if keyword == "" {
		return "", fmt.Errorf("keyword is required")
	}
	if limit <= 0 {
		limit = 4
	}
	if limit > 16 {
		limit = 16
	}

	// configDir, err := utils.GetConfigDir("errors")
	// if err != nil {
	// 	return "", fmt.Errorf("utils.GetConfigDir: %w", err)
	// }

	index := getErrorList(filesystem.ErrorsDir)
	if len(index) == 0 {
		return "NONE", nil
	}

	lower := strings.ToLower(keyword)
	var matched []ErrorMemory
	for tool := range index {
		jsonPath := filepath.Join(filesystem.ErrorsDir, tool+".json")
		records := getErrorMemory(jsonPath)
		for _, record := range records {
			if matchErrorMemory(record, lower) {
				matched = append(matched, record)
			}
		}
	}

	if len(matched) == 0 {
		return "NONE", nil
	}

	return formatRecords(matched, limit), nil
}

func getErrorList(home string) map[string]ErrorMemoryItem {
	index := make(map[string]ErrorMemoryItem)
	errorDir := filepath.Join(home, "errors.json")
	data, err := os.ReadFile(errorDir)
	if err != nil {
		return index
	}
	err = json.Unmarshal(data, &index)
	if err != nil {
		return index
	}
	return index
}

func getErrorMemory(path string) []ErrorMemory {
	var datas []ErrorMemory
	data, err := os.ReadFile(path)
	if err != nil {
		return datas
	}
	err = json.Unmarshal(data, &datas)
	if err != nil {
		return datas
	}
	return datas
}

func matchErrorMemory(rec ErrorMemory, lower string) bool {
	if strings.Contains(strings.ToLower(rec.ToolName), lower) ||
		strings.Contains(strings.ToLower(rec.Symptom), lower) ||
		strings.Contains(strings.ToLower(rec.Cause), lower) {
		return true
	}
	for _, keyword := range rec.Keywords {
		text := strings.ToLower(keyword)
		if strings.Contains(text, lower) || strings.Contains(lower, text) {
			return true
		}
	}
	return false
}

func formatRecords(records []ErrorMemory, limit int) string {
	start := max(0, len(records)-limit)
	slice := make([]ErrorMemory, 0, limit)
	for i := len(records) - 1; i >= start; i-- {
		slice = append(slice, records[i])
	}
	out, err := json.Marshal(slice)
	if err != nil {
		return ""
	}
	return string(out)
}

func SaveErrorMemory(sessionID string, record ErrorMemory) (string, error) {
	// configDir, err := utils.GetConfigDir("errors")
	// if err != nil {
	// 	return "", fmt.Errorf("utils.GetConfigDir: %w", err)
	// }

	now := time.Now()
	h := sha256.Sum256([]byte(record.ToolName + strconv.FormatInt(now.Unix(), 10)))
	record.ID = hex.EncodeToString(h[:])
	record.Timestamp = now.Unix()

	jsonPath := filepath.Join(filesystem.ErrorsDir, record.ToolName+".json")
	records := getErrorMemory(jsonPath)
	records = append(records, record)
	data, err := json.Marshal(records)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	if err := filesystem.WriteFile(jsonPath, string(data), 0644); err != nil {
		return "", fmt.Errorf("utils.WriteFile: %w", err)
	}

	index := getErrorList(filesystem.ErrorsDir)
	index[record.ToolName] = ErrorMemoryItem{
		Count:       len(records),
		LastUpdated: now.Unix(),
	}
	writeErrorList(filesystem.ErrorsDir, index)

	return fmt.Sprintf("Remember the Error: %s", record.ID), nil
}

func writeErrorList(home string, index map[string]ErrorMemoryItem) error {
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}

	if err = filesystem.WriteFile(filepath.Join(home, "errors.json"), string(data), 0644); err != nil {
		return err
	}
	return nil
}

func SearchErrorMemory(tool, keyword string, limit int) string {
	result, err := searchFile(tool, keyword, limit)
	if err != nil {
		slog.Warn("searchFile",
			slog.String("error", err.Error()))
		return ""
	}
	if result == "NONE" {
		result, err = searchFile(tool, "", limit)
		if err != nil || result == "NONE" {
			return ""
		}
	}
	return result
}

func searchFile(toolName, keyword string, limit int) (string, error) {
	if limit <= 0 {
		limit = 4
	}
	if limit > 16 {
		limit = 16
	}

	// configDir, err := utils.GetConfigDir("errors")
	// if err != nil {
	// 	return "", fmt.Errorf("utils.GetConfigDir: %w", err)
	// }

	jsonPath := filepath.Join(filesystem.ErrorsDir, toolName+".json")
	records := getErrorMemory(jsonPath)
	if len(records) == 0 {
		return "NONE", nil
	}

	lower := strings.ToLower(keyword)
	var matched []ErrorMemory
	for _, record := range records {
		if matchErrorMemory(record, lower) {
			matched = append(matched, record)
		}
	}

	if len(matched) == 0 {
		return "NONE", nil
	}

	return formatRecords(matched, limit), nil
}
