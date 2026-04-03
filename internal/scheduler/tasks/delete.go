package tasks

import (
	"fmt"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/script"
)

func Delete(s *scheduler.Scheduler, id string) error {
	if s == nil {
		items, err := filesystem.GetTasks()
		if err != nil {
			return fmt.Errorf("filesystem.GetTasks: %w", err)
		}
		var target *filesystem.TaskItem
		kept := make([]filesystem.TaskItem, 0, len(items))
		for _, item := range items {
			if item.ID == id {
				cp := item
				target = &cp
			} else {
				kept = append(kept, item)
			}
		}
		if target == nil {
			return fmt.Errorf("not found: %s", id)
		}
		if err := filesystem.WriteTasks(kept); err != nil {
			return fmt.Errorf("filesystem.WriteTasks: %w", err)
		}
		script.Remove(filepath.Join(filesystem.ScriptsDir, target.Script))
		_ = filesystem.DeleteTaskResult(target.ID, target.At)
		return nil
	}

	s.Mu.Lock()
	idx, target := fit(s, id)
	if idx == -1 {
		s.Mu.Unlock()
		return fmt.Errorf("not found: %s", id)
	}
	s.Tasks = append(s.Tasks[:idx], s.Tasks[idx+1:]...)
	snapshot := make([]filesystem.TaskItem, len(s.Tasks))
	copy(snapshot, s.Tasks)
	s.Mu.Unlock()

	_ = filesystem.WriteTasks(snapshot)
	script.Remove(filepath.Join(filesystem.ScriptsDir, target.Script))
	_ = filesystem.DeleteTaskResult(target.ID, target.At)
	return nil
}
