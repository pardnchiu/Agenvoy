package crons

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
)

func Setup(s *scheduler.Scheduler) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	items, err := filesystem.GetCrons()
	if err != nil {
		return fmt.Errorf("filesystem.GetCrons: %w", err)
	}

	var valid []filesystem.CronItem
	var crons []filesystem.CronItem
	for _, item := range items {
		scriptPath := filepath.Join(filesystem.ScriptsDir, item.Script)
		if _, err := os.Stat(scriptPath); err != nil {
			slog.Warn("os.Stat",
				slog.String("id", item.ID),
				slog.String("script", item.Script))
			_ = filesystem.DeleteCronResult(item.ID)
			continue
		}
		valid = append(valid, item)
		id, err := s.Cron.Add(item.Expression, set(s, item))
		if err != nil {
			slog.Warn("s.cron.Add",
				slog.String("error", err.Error()))
			continue
		}
		item.CronID = id
		crons = append(crons, item)
	}
	if len(valid) < len(items) {
		if err := filesystem.WriteCrons(valid); err != nil {
			return fmt.Errorf("filesystem.WriteCrons: %w", err)
		}
	}
	s.Crons = crons

	results, err := filesystem.GetAllCronResults()
	if err != nil {
		return fmt.Errorf("filesystem.GetAllCronResults: %w", err)
	}
	for _, r := range results {
		s.CronResults[r.ID] = r
	}

	return nil
}
