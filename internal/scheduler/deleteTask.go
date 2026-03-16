package scheduler

import (
	"fmt"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (s *Scheduler) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, t := range s.tasks {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("not found: %s", id)
	}

	target := s.tasks[idx]
	if timer, ok := s.timers[target.ID]; ok {
		timer.Stop()
		delete(s.timers, target.ID)
	}

	tasks, err := filesystem.GetTasks()
	if err != nil {
		return fmt.Errorf("filesystem.GetTasks: %w", err)
	}
	var kept []filesystem.TaskItem
	for _, t := range tasks {
		if t.ID != id {
			kept = append(kept, t)
		}
	}
	if err := filesystem.WriteTasks(kept); err != nil {
		return fmt.Errorf("filesystem.WriteTasks: %w", err)
	}

	s.tasks = append(s.tasks[:idx], s.tasks[idx+1:]...)
	removeScript(filepath.Join(filesystem.ScriptsDir, target.Script))
	return nil
}
