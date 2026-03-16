package scheduler

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (s *Scheduler) SetupTasks() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := filesystem.GetTasks()
	if err != nil {
		return fmt.Errorf("filesystem.GetTasks: %w", err)
	}

	now := time.Now()
	var valid []filesystem.TaskItem
	for _, item := range items {
		if !item.At.After(now) {
			continue
		}
		valid = append(valid, item)
		if err := s.setTask(item); err != nil {
			slog.Warn("s.setTask",
				slog.String("error", err.Error()))
		}
	}

	// * rewrite file with only future tasks
	if err := filesystem.WriteTasks(valid); err != nil {
		return fmt.Errorf("filesystem.WriteTasks: %w", err)
	}
	return nil
}

func (s *Scheduler) ListTasks() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]string, len(s.tasks))
	for i, task := range s.tasks {
		result[i] = fmt.Sprintf("%s %s %s", task.ID, task.At.Local().Format("2006-01-02 15:04:05"), task.Script)
	}
	return result
}
