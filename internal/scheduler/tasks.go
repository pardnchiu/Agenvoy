package scheduler

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (s *Scheduler) LoadTasks() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines, err := filesystem.ReadFileSlice(filesystem.TasksPath)
	if err != nil {
		return fmt.Errorf("filesystem.ReadFile: %w", err)
	}

	now := time.Now()
	var skip []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}

		item, err := parseTaskLine(trim)
		if err != nil {
			slog.Warn("parseTaskLine",
				slog.String("error", err.Error()))
			continue
		}

		if !item.at.After(now) {
			continue
		}

		skip = append(skip, trim)
		s.setTask(item)
	}

	return filesystem.WriteFileWithLines(filesystem.TasksPath, skip, 0644)
}

func (s *Scheduler) RemoveTask(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 1 || index > len(s.tasks) {
		return fmt.Errorf("not exist")
	}

	target := s.tasks[index-1]

	if timer, ok := s.timers[target.line]; ok {
		timer.Stop()
		delete(s.timers, target.line)
	}
	removeLine(filesystem.TasksPath, target.line)
	s.tasks = append(s.tasks[:index-1], s.tasks[index:]...)
	removeScript(filepath.Join(filesystem.ScriptsDir, target.script))
	return nil
}

func (s *Scheduler) ListTasks() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]string, len(s.tasks))
	for i, task := range s.tasks {
		result[i] = fmt.Sprintf("%d. %s", i+1, task.line)
	}
	return result
}
