package tasks

import (
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
)

func ListTasks(s *scheduler.Scheduler) ([]string, error) {
	if s == nil {
		items, err := filesystem.GetTasks()
		if err != nil {
			return nil, err
		}
		result := make([]string, len(items))
		for i, task := range items {
			result[i] = fmt.Sprintf("%s %s %s", task.ID, task.At.Local().Format("2006-01-02 15:04:05"), task.Script)
		}
		return result, nil
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	result := make([]string, len(s.Tasks))
	for i, task := range s.Tasks {
		result[i] = fmt.Sprintf("%s %s %s", task.ID, task.At.Local().Format("2006-01-02 15:04:05"), task.Script)
	}
	return result, nil
}
