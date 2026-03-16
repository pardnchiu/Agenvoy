package scheduler

import (
	"fmt"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (s *Scheduler) UpdateTask(id, timeText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, t := range s.tasks {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("not found: %s", id)
	}

	at, err := parseTaskTime(timeText)
	if err != nil {
		return err
	}
	if !at.After(time.Now()) {
		return fmt.Errorf("already gone")
	}

	old := s.tasks[idx]
	if timer, ok := s.timers[old.ID]; ok {
		timer.Stop()
		delete(s.timers, old.ID)
	}

	updated := filesystem.TaskItem{
		ID:        old.ID,
		At:        at.UTC(),
		Script:    old.Script,
		ChannelID: old.ChannelID,
	}

	tasks, err := filesystem.GetTasks()
	if err != nil {
		return fmt.Errorf("filesystem.GetTasks: %w", err)
	}
	for i, t := range tasks {
		if t.ID == id {
			tasks[i].At = updated.At
			break
		}
	}

	if err := filesystem.WriteTasks(tasks); err != nil {
		return fmt.Errorf("filesystem.WriteTasks: %w", err)
	}

	s.tasks = append(s.tasks[:idx], s.tasks[idx+1:]...)
	return s.setTask(updated)
}
