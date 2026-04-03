package crons

import (
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
)

func Update(s *scheduler.Scheduler, id, expression string) error {
	if len(strings.Fields(expression)) != 5 {
		return fmt.Errorf("expression must be 5 fields `{min} {hour} {dom} {mon} {dow}`")
	}

	if s == nil {
		items, err := filesystem.GetCrons()
		if err != nil {
			return fmt.Errorf("filesystem.GetCrons: %w", err)
		}
		idx := -1
		for i, c := range items {
			if c.ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("not found: %s", id)
		}
		items[idx].Expression = expression
		return filesystem.WriteCrons(items)
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	idx, target := fitTarget(s, id)
	if idx == -1 {
		return fmt.Errorf("not found: %s", id)
	}

	newTarget := filesystem.CronItem{
		ID:        target.ID,
		Expression: expression,
		Script:    target.Script,
		ChannelID: target.ChannelID,
	}

	newID, err := s.Cron.Add(newTarget.Expression, set(s, newTarget))
	if err != nil {
		if oldID, e := s.Cron.Add(target.Expression, set(s, target)); e == nil {
			s.Crons[idx].CronID = oldID
		} else {
			s.Crons = append(s.Crons[:idx], s.Crons[idx+1:]...)
			kept := make([]filesystem.CronItem, len(s.Crons))
			copy(kept, s.Crons)
			_ = filesystem.WriteCrons(kept)
		}
		return fmt.Errorf("cron.Add: %w", err)
	}
	newTarget.CronID = newID

	updated := make([]filesystem.CronItem, len(s.Crons))
	copy(updated, s.Crons)
	updated[idx] = newTarget

	if err := filesystem.WriteCrons(updated); err != nil {
		return fmt.Errorf("filesystem.WriteCrons: %w", err)
	}

	s.Crons[idx] = newTarget
	return nil
}

func fitTarget(s *scheduler.Scheduler, id string) (int, filesystem.CronItem) {
	idx := -1
	for i, cron := range s.Crons {
		if cron.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return -1, filesystem.CronItem{}
	}

	target := s.Crons[idx]
	s.Cron.Remove(target.CronID)

	return idx, target
}
