package tasks

import (
	"fmt"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func AddToFile(text, script, channelID string) (string, error) {
	at, err := parseTime(text)
	if err != nil {
		return "", err
	}
	if !at.After(time.Now()) {
		return "", fmt.Errorf("already gone")
	}
	item := filesystem.TaskItem{
		ID:        utils.NewID(at.UTC().Format(time.RFC3339), script),
		At:        at.UTC(),
		Script:    script,
		ChannelID: channelID,
	}
	existing, _ := filesystem.GetTasks()
	if err := filesystem.WriteTasks(append(existing, item)); err != nil {
		return "", fmt.Errorf("filesystem.WriteTasks: %w", err)
	}
	_ = filesystem.WriteTaskResult(filesystem.TaskResult{
		ID:     item.ID,
		At:     item.At,
		Script: item.Script,
		Status: "pending",
	})
	return fmt.Sprintf("task added: scheduled at %s for %s\n-# ID: `%s`",
		at.Local().Format("2006-01-02 15:04:05"), script, item.ID), nil
}

// * allow: +5m, +1h30m, 15:04, 2006-01-02 15:04, RFC3339
func Add(s *scheduler.Scheduler, text, script, channelID string) (string, error) {
	at, err := parseTime(text)
	if err != nil {
		return "", err
	}

	if !at.After(time.Now()) {
		return "", fmt.Errorf("already gone")
	}

	item := filesystem.TaskItem{
		ID:        utils.NewID(at.UTC().Format(time.RFC3339), script),
		At:        at.UTC(),
		Script:    script,
		ChannelID: channelID,
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	if err := filesystem.WriteTasks(append(s.Tasks, item)); err != nil {
		return "", fmt.Errorf("filesystem.WriteTasks: %w", err)
	}

	_ = filesystem.WriteTaskResult(filesystem.TaskResult{
		ID:     item.ID,
		At:     item.At,
		Script: item.Script,
		Status: "pending",
	})

	if err := Set(s, item); err != nil {
		return "", fmt.Errorf("SetTask: %w", err)
	}
	return fmt.Sprintf("task added: scheduled at %s for %s\n-# ID: `%s`",
		at.Local().Format("2006-01-02 15:04:05"), script, item.ID), nil
}
