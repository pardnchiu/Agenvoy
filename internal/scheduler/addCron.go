package scheduler

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (s *Scheduler) AddCron(expression, script, channelID string) (string, error) {
	if len(strings.Fields(expression)) != 5 {
		return "", fmt.Errorf("expression must be 5 fields `{min} {hour} {dom} {mon} {dow}`")
	}

	item := filesystem.CronItem{
		ID:         newID(expression, script),
		Expression: expression,
		Script:     script,
		ChannelID:  channelID,
	}

	id, err := s.cron.Add(item.Expression, s.makeCronAction(item))
	if err != nil {
		return "", fmt.Errorf("s.cron.Add: %w", err)
	}
	item.CronID = id

	s.mu.Lock()
	defer s.mu.Unlock()

	crons, err := filesystem.GetCrons()
	if err != nil {
		s.cron.Remove(id)
		return "", fmt.Errorf("readCronsJSON: %w", err)
	}

	if err := filesystem.WriteCrons(append(crons, item)); err != nil {
		s.cron.Remove(id)
		return "", fmt.Errorf("filesystem.WriteCrons: %w", err)
	}

	s.crons = append(s.crons, item)
	return fmt.Sprintf("cron task added: %s %s\n-# ID: `%s`", expression, script, item.ID), nil
}

func (s *Scheduler) makeCronAction(item filesystem.CronItem) func() {
	return func() {
		output := runScript("cron", filepath.Join(filesystem.ScriptsDir, item.Script))
		s.mu.Lock()
		cb := s.OnCompleted
		s.mu.Unlock()
		if item.ChannelID != "" && cb != nil {
			cb(item.ChannelID, output)
		}
	}
}
