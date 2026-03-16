package scheduler

import (
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (s *Scheduler) UpdateCron(id, expression string) error {
	if len(strings.Fields(expression)) != 5 {
		return fmt.Errorf("expression must be 5 fields `{min} {hour} {dom} {mon} {dow}`")
	}

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

	old := s.crons[idx]
	s.cron.Remove(old.CronID)

	updated := filesystem.CronItem{
		ID:         old.ID,
		Expression: expression,
		Script:     old.Script,
		ChannelID:  old.ChannelID,
	}

	newID, err := s.cron.Add(updated.Expression, s.makeCronAction(updated))
	if err != nil {
		return fmt.Errorf("cron.Add: %w", err)
	}
	updated.CronID = newID

	crons, err := filesystem.GetCrons()
	if err != nil {
		return fmt.Errorf("filesystem.GetCrons: %w", err)
	}
	for i, c := range crons {
		if c.ID == id {
			crons[i].Expression = expression
			break
		}
	}

	if err := filesystem.WriteCrons(crons); err != nil {
		return fmt.Errorf("filesystem.writeCrons: %w", err)
	}

	s.crons[idx] = updated
	return nil
}
