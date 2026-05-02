package session

import (
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

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
		ID:        go_pkg_utils.UUID(),
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
	if !go_pkg_filesystem_reader.Exists(dir) {
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
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return
	}
	for _, dir := range dirs {
		ClearTask(dir.Name)
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
	s, err := go_pkg_filesystem.ReadJSON[Status](path)
	if err != nil {
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
	if !go_pkg_filesystem_reader.Exists(dir) {
		return
	}
	if s.Active == nil {
		s.Active = []Task{}
	}
	path := filepath.Join(dir, "status.json")
	if err := go_pkg_filesystem.WriteJSON(path, s, true); err != nil {
		slog.Warn("go_pkg_filesystem.WriteJSON",
			slog.String("error", err.Error()))
	}
}
