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
