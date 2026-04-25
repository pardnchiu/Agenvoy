package tasks

import (
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/script"
)

func Set(s *scheduler.Scheduler, item filesystem.TaskItem) error {
	scriptPath := filepath.Join(filesystem.ScriptsDir, item.Script)
	delay := time.Until(item.At)

	timer := time.AfterFunc(delay, func() {
		now := time.Now()
		running := filesystem.TaskResult{
			ID:        item.ID,
			At:        item.At,
			Script:    item.Script,
			Status:    "running",
			StartedAt: &now,
		}
		s.Mu.Lock()
		s.TaskResults[item.ID] = running
		s.Mu.Unlock()
		_ = filesystem.WriteTaskResult(running)

		output := script.Run("task", scriptPath)
		fin := time.Now()

		status := "completed"
		outVal, errVal := output, ""
		if strings.HasPrefix(output, "error:") {
			status = "failed"
			outVal, errVal = "", output
		}

		result := filesystem.TaskResult{
			ID:         item.ID,
			At:         item.At,
			Script:     item.Script,
			Status:     status,
			StartedAt:  &now,
			FinishedAt: &fin,
			Output:     outVal,
			Err:        errVal,
		}

		s.Mu.Lock()
		s.TaskResults[item.ID] = result
		delete(s.Timers, item.ID)
		for i := range s.Tasks {
			if s.Tasks[i].ID == item.ID {
				s.Tasks = append(s.Tasks[:i], s.Tasks[i+1:]...)
				break
			}
		}
		snapshot := make([]filesystem.TaskItem, len(s.Tasks))
		copy(snapshot, s.Tasks)
		cb := s.OnCompleted
		s.Mu.Unlock()

		if err := filesystem.WriteTaskResult(result); err != nil {
			slog.Warn("filesystem.WriteTaskResult",
				slog.String("error", err.Error()))
		}
		_ = filesystem.WriteTasks(snapshot)
		script.Remove(scriptPath)

		if item.ChannelID != "" && cb != nil {
			cb(item.ChannelID, output)
		}
	})

	s.Timers[item.ID] = timer
	s.Tasks = append(s.Tasks, item)
	return nil
}
