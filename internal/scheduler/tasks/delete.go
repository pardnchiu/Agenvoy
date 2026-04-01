package tasks

import (
	"fmt"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/script"
)

func Delete(s *scheduler.Scheduler, id string) error {
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
