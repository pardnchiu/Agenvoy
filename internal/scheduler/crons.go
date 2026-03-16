package scheduler

import (
	"fmt"
	"log/slog"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (s *Scheduler) SetupCrons() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := filesystem.GetCrons()
	if err != nil {
		return fmt.Errorf("filesystem.GetCrons: %w", err)
	}

	var crons []filesystem.CronItem
	for _, item := range items {
		id, err := s.cron.Add(item.Expression, s.makeCronAction(item))
		if err != nil {
			slog.Warn("s.cron.Add",
				slog.String("error", err.Error()))
			continue
		}
		item.CronID = id
		crons = append(crons, item)
	}

	s.crons = crons
	return nil
}

func (s *Scheduler) ListCrons() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]string, len(s.crons))
	for i, t := range s.crons {
		result[i] = fmt.Sprintf("%s %s %s", t.ID, t.Expression, t.Script)
	}
	return result
}
