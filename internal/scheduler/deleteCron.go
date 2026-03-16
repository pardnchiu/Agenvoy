package scheduler

import (
	"fmt"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (s *Scheduler) DeleteCron(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, c := range s.crons {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("not found: %s", id)
	}

	target := s.crons[idx]
	s.cron.Remove(target.CronID)

	crons, err := filesystem.GetCrons()
	if err != nil {
		return fmt.Errorf("filesystem.GetCrons: %w", err)
	}

	var kept []filesystem.CronItem
	for _, c := range crons {
		if c.ID != id {
			kept = append(kept, c)
		}
	}

	if err := filesystem.WriteCrons(kept); err != nil {
		return fmt.Errorf("filesystem.WriteCrons: %w", err)
	}

	s.crons = append(s.crons[:idx], s.crons[idx+1:]...)
	removeScript(filepath.Join(filesystem.ScriptsDir, target.Script))
	return nil
}
