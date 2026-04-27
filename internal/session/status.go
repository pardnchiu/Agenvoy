package session

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	go_utils_utils "github.com/pardnchiu/go-utils/utils"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const (
	StatusOnline      = "online"
	StatusIdle        = "idle"
	statusInputMaxLen = 256
)

type Task struct {
	ID        string `json:"id"`
	Input     string `json:"input"`
	StartedAt string `json:"started_at"`
}

type Status struct {
	State   string `json:"state"`
	Active  []Task `json:"active"`
	EndedAt string `json:"ended_at"`
}

var statusMu sync.Mutex

func Online(sessionID, input string) string {
	if sessionID == "" {
		return ""
	}
	statusMu.Lock()
	defer statusMu.Unlock()

	s := readStatus(sessionID)
	task := Task{
		ID:        go_utils_utils.UUID(),
		Input:     truncateStatusInput(input),
		StartedAt: nowStatusTime(),
	}
	s.Active = append(s.Active, task)
	s.State = StatusOnline
	writeStatus(sessionID, s)
	return task.ID
}

func Idle(sessionID, taskID string) {
	if sessionID == "" || taskID == "" {
		return
	}
	statusMu.Lock()
	defer statusMu.Unlock()

	s := readStatus(sessionID)
	filtered := s.Active[:0]
	for _, t := range s.Active {
		if t.ID == taskID {
			continue
		}
		filtered = append(filtered, t)
	}
	s.Active = filtered
	if len(s.Active) == 0 {
		s.State = StatusIdle
		s.EndedAt = nowStatusTime()
	} else {
		s.State = StatusOnline
	}
	writeStatus(sessionID, s)
}

func ReadStatus(sessionID string) Status {
	if sessionID == "" {
		return Status{}
	}
	statusMu.Lock()
	defer statusMu.Unlock()
	return readStatus(sessionID)
}

func ClearTask(sessionID string) {
	if sessionID == "" {
		return
	}
	statusMu.Lock()
	defer statusMu.Unlock()

	dir := filepath.Join(filesystem.SessionsDir, sessionID)
	if _, err := os.Stat(dir); err != nil {
		return
	}
	s := readStatus(sessionID)
	if len(s.Active) == 0 && s.State != StatusOnline {
		return
	}
	s.Active = nil
	s.State = StatusIdle
	s.EndedAt = nowStatusTime()
	writeStatus(sessionID, s)
}

func CleanAllTask() {
	entries, err := os.ReadDir(filesystem.SessionsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ClearTask(entry.Name())
	}
}

func nowStatusTime() string {
	return time.Now().Format("2006-01-02 15:04:05.000")
}

func truncateStatusInput(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= statusInputMaxLen {
		return s
	}
	return s[:statusInputMaxLen] + "…[truncated]"
}

func readStatus(sessionID string) Status {
	path := filepath.Join(filesystem.SessionsDir, sessionID, "status.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Status{State: StatusIdle}
	}
	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		return Status{State: StatusIdle}
	}
	if s.State == "" {
		if len(s.Active) > 0 {
			s.State = StatusOnline
		} else {
			s.State = StatusIdle
		}
	}
	return s
}

func writeStatus(sessionID string, s Status) {
	dir := filepath.Join(filesystem.SessionsDir, sessionID)
	if _, err := os.Stat(dir); err != nil {
		return
	}
	if s.Active == nil {
		s.Active = []Task{}
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		slog.Warn("json.MarshalIndent",
			slog.String("error", err.Error()))
		return
	}
	path := filepath.Join(dir, "status.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		slog.Warn("os.WriteFile",
			slog.String("error", err.Error()))
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		slog.Warn("os.Rename",
			slog.String("error", err.Error()))
		_ = os.Remove(tmp)
	}
}
