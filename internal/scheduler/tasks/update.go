package tasks

import (
	"fmt"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
)

func fit(s *scheduler.Scheduler, id string) (int, filesystem.TaskItem) {
	idx := -1
	for i, t := range s.Tasks {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return -1, filesystem.TaskItem{}
	}

	target := s.Tasks[idx]
	if timer, ok := s.Timers[target.ID]; ok {
		timer.Stop()
		delete(s.Timers, target.ID)
	}

	return idx, target
}

func Update(s *scheduler.Scheduler, id, timeText string) error {
	at, err := parseTime(timeText)
	if err != nil {
		return err
	}
	if !at.After(time.Now()) {
		return fmt.Errorf("already gone")
	}

	if s == nil {
		items, err := filesystem.GetTasks()
		if err != nil {
			return fmt.Errorf("filesystem.GetTasks: %w", err)
		}
		idx := -1
		for i, t := range items {
			if t.ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("not found: %s", id)
		}
		items[idx].At = at.UTC()
		return filesystem.WriteTasks(items)
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	idx, target := fit(s, id)
	if idx == -1 {
		return fmt.Errorf("not found: %s", id)
	}

	newTask := filesystem.TaskItem{
		ID:        target.ID,
		At:        at.UTC(),
		Script:    target.Script,
		ChannelID: target.ChannelID,
	}

	updated := make([]filesystem.TaskItem, len(s.Tasks))
	copy(updated, s.Tasks)
	updated[idx] = newTask

	if err := filesystem.WriteTasks(updated); err != nil {
		return fmt.Errorf("filesystem.WriteTasks: %w", err)
	}

	s.Tasks = append(s.Tasks[:idx], s.Tasks[idx+1:]...)
	return Set(s, newTask)
}
