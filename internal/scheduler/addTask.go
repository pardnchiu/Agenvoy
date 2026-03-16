package scheduler

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

// * allow: +5m, +1h30m, 15:04, 2006-01-02 15:04, RFC3339
func (s *Scheduler) AddTask(text, script, channelID string) (string, error) {
	at, err := parseTaskTime(text)
	if err != nil {
		return "", err
	}

	if !at.After(time.Now()) {
		return "", fmt.Errorf("already gone")
	}

	item := filesystem.TaskItem{
		ID:        newID(at.UTC().Format(time.RFC3339), script),
		At:        at.UTC(),
		Script:    script,
		ChannelID: channelID,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := filesystem.GetTasks()
	if err != nil {
		return "", fmt.Errorf("filesystem.GetTasks: %w", err)
	}

	if err := filesystem.WriteTasks(append(tasks, item)); err != nil {
		return "", fmt.Errorf("filesystem.WriteTasks: %w", err)
	}

	if err := s.setTask(item); err != nil {
		return "", fmt.Errorf("s.setTask: %w", err)
	}
	return fmt.Sprintf("task added: scheduled at %s for %s\n-# ID: `%s`", at.Local().Format("2006-01-02 15:04:05"), script, item.ID), nil
}

func parseTaskTime(text string) (time.Time, error) {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "+") {
		duration, err := time.ParseDuration(text[1:])
		if err != nil {
			return time.Time{}, fmt.Errorf("time.ParseDuration: %w", err)
		}
		return time.Now().Add(duration), nil
	}

	if t, err := time.ParseInLocation("2006-01-02 15:04", text, time.Local); err == nil {
		return t, nil
	}

	if t, err := time.Parse(time.RFC3339, text); err == nil {
		return t, nil
	}

	// * 15:04, no date, assume today
	if t, err := time.ParseInLocation("15:04", text, time.Local); err == nil {
		now := time.Now()
		result := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		if !result.After(now) {
			return time.Time{}, fmt.Errorf("already gone: %q", text)
		}
		return result, nil
	}

	return time.Time{}, fmt.Errorf("parseTime: %s", text)
}

func (s *Scheduler) setTask(item filesystem.TaskItem) error {
	scriptPath := filepath.Join(filesystem.ScriptsDir, item.Script)
	delay := time.Until(item.At)

	execTime := time.AfterFunc(delay, func() {
		output := runScript("task", scriptPath)

		if item.ChannelID != "" {
			s.mu.Lock()
			cb := s.OnCompleted
			s.mu.Unlock()

			if cb != nil {
				cb(item.ChannelID, output)
			}
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		delete(s.timers, item.ID)
		removeTaskFromJSON(item.ID)
		s.removeTaskByID(item.ID)
		removeScript(scriptPath)
	})
	s.timers[item.ID] = execTime
	s.tasks = append(s.tasks, item)
	return nil
}

func (s *Scheduler) removeTaskByID(id string) {
	for i, task := range s.tasks {
		if task.ID == id {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			return
		}
	}
}

func removeTaskFromJSON(id string) {
	tasks, err := filesystem.GetTasks()
	if err != nil {
		return
	}
	var kept []filesystem.TaskItem
	for _, t := range tasks {
		if t.ID != id {
			kept = append(kept, t)
		}
	}
	filesystem.WriteTasks(kept)
}
