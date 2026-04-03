package crons

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/script"
)

func Delete(s *scheduler.Scheduler, id string) error {
	if s == nil {
		items, err := filesystem.GetCrons()
		if err != nil {
			return fmt.Errorf("filesystem.GetCrons: %w", err)
		}
		var target *filesystem.CronItem
		kept := make([]filesystem.CronItem, 0, len(items))
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
		if err := filesystem.WriteCrons(kept); err != nil {
			return fmt.Errorf("filesystem.WriteCrons: %w", err)
		}
		script.Remove(filepath.Join(filesystem.ScriptsDir, target.Script))
		_ = filesystem.DeleteCronResult(id)
		return nil
	}

	s.Mu.Lock()
	idx, target := fitTarget(s, id)
	if idx == -1 {
		s.Mu.Unlock()
		return fmt.Errorf("not found: %s", id)
	}
	kept := make([]filesystem.CronItem, 0, len(s.Crons)-1)
	for _, c := range s.Crons {
		if c.ID != id {
			kept = append(kept, c)
		}
	}
	s.Crons = append(s.Crons[:idx], s.Crons[idx+1:]...)
	delete(s.CronResults, id)
	s.Mu.Unlock()

	_ = filesystem.WriteCrons(kept)
	script.Remove(filepath.Join(filesystem.ScriptsDir, target.Script))
	if err := filesystem.DeleteCronResult(id); err != nil {
		slog.Warn("filesystem.DeleteCronResult",
			slog.String("id", id),
			slog.String("error", err.Error()))
	}
	return nil
}
