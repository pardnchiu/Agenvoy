package tasks

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
)

func Setup(s *scheduler.Scheduler) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	items, err := filesystem.GetTasks()
	if err != nil {
		return fmt.Errorf("filesystem.GetTasks: %w", err)
	}

	now := time.Now()
	var pending []filesystem.TaskItem
	for _, item := range items {
		if !item.At.After(now) {
			continue
		}
		scriptPath := filepath.Join(filesystem.ScriptsDir, item.Script)
		if !go_pkg_filesystem_reader.Exists(scriptPath) {
			slog.Warn("SetupTasks: script not found, removing task",
				slog.String("id", item.ID),
				slog.String("script", item.Script))
			_ = filesystem.DeleteTaskResult(item.ID, item.At)
			continue
		}
		pending = append(pending, item)
		if err := Set(s, item); err != nil {
			slog.Warn("s.setTask", slog.String("error", err.Error()))
		}
	}
	if len(pending) < len(items) {
		if err := filesystem.WriteTasks(pending); err != nil {
			return fmt.Errorf("filesystem.WriteTasks: %w", err)
		}
	}

	results, err := filesystem.GetAllTaskResults()
	if err != nil {
		return fmt.Errorf("filesystem.GetAllTaskResults: %w", err)
	}
	for _, r := range results {
		if r.Status == "running" {
			r.Status = "failed"
			r.Err = "interrupted by restart"
			_ = filesystem.WriteTaskResult(r)
		}
		s.TaskResults[r.ID] = r
	}

	return nil
}

func GetTask(s *scheduler.Scheduler, id string) (*filesystem.TaskResult, bool) {
	if s == nil {
		results, err := filesystem.GetAllTaskResults()
		if err != nil {
			return nil, false
		}
		for _, r := range results {
			if r.ID == id {
				cp := r
				return &cp, true
			}
		}
		return nil, false
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	r, ok := s.TaskResults[id]
	if !ok {
		return nil, false
	}
	cp := r
	return &cp, true
}
