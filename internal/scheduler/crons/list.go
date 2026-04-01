package crons

import (
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
)

func List(s *scheduler.Scheduler) []string {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	results := make([]string, len(s.Crons))
	for i, cron := range s.Crons {
		line := fmt.Sprintf("%s %s %s", cron.ID, cron.Expression, cron.Script)
		if result, ok := s.CronResults[cron.ID]; ok {
			line += fmt.Sprintf(" [last: %s]", result.Status)
		}
		results[i] = line
	}
	return results
}
