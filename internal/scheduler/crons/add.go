package crons

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/script"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func AddToFile(expression, script, channelID string) (string, error) {
	if len(strings.Fields(expression)) != 5 {
		return "", fmt.Errorf("expression must be 5 fields `{min} {hour} {dom} {mon} {dow}`")
	}
	item := filesystem.CronItem{
		ID:         utils.NewID(expression, script),
		Expression: expression,
		Script:     script,
		ChannelID:  channelID,
	}
	existing, _ := filesystem.GetCrons()
	if err := filesystem.WriteCrons(append(existing, item)); err != nil {
		return "", fmt.Errorf("filesystem.WriteCrons: %w", err)
	}
	_ = filesystem.WriteCronResult(filesystem.CronResult{ID: item.ID, Status: "pending"})
	return fmt.Sprintf("cron task added: %s %s\n-# ID: `%s`", expression, script, item.ID), nil
}

func Add(s *scheduler.Scheduler, expression, script, channelID string) (string, error) {
	if len(strings.Fields(expression)) != 5 {
		return "", fmt.Errorf("expression must be 5 fields `{min} {hour} {dom} {mon} {dow}`")
	}

	item := filesystem.CronItem{
		ID:         utils.NewID(expression, script),
		Expression: expression,
		Script:     script,
		ChannelID:  channelID,
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	id, err := s.Cron.Add(item.Expression, set(s, item))
	if err != nil {
		return "", fmt.Errorf("s.cron.Add: %w", err)
	}
	item.CronID = id

	if err := filesystem.WriteCrons(append(s.Crons, item)); err != nil {
		s.Cron.Remove(id)
		return "", fmt.Errorf("filesystem.WriteCrons: %w", err)
	}

	_ = filesystem.WriteCronResult(filesystem.CronResult{
		ID:     item.ID,
		Status: "pending",
	})

	s.Crons = append(s.Crons, item)
	return fmt.Sprintf("cron task added: %s %s\n-# ID: `%s`", expression, script, item.ID), nil
}

func set(s *scheduler.Scheduler, item filesystem.CronItem) func() {
	return func() {
		output := script.Run("cron", filepath.Join(filesystem.ScriptsDir, item.Script))
		runAt := time.Now()

		status := "completed"
		outVal, errVal := output, ""
		if strings.HasPrefix(output, "error:") {
			status = "failed"
			outVal, errVal = "", output
		}

		result := filesystem.CronResult{
			ID:     item.ID,
			RunAt:  &runAt,
			Status: status,
			Output: outVal,
			Err:    errVal,
		}

		s.Mu.Lock()
		s.CronResults[item.ID] = result
		cb := s.OnCompleted
		s.Mu.Unlock()

		// TODO: handle error
		_ = filesystem.WriteCronResult(result)
		_ = filesystem.WriteCronRecord(result)

		if item.ChannelID != "" && cb != nil {
			cb(item.ChannelID, output)
		}
	}
}
