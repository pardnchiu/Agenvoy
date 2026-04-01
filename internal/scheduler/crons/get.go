package crons

import (
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
)

func GetCron(s *scheduler.Scheduler, id string) (*filesystem.CronItem, *filesystem.CronResult, bool) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	for _, cron := range s.Crons {
		if cron.ID == id {
			item := cron
			var rp *filesystem.CronResult
			if result, ok := s.CronResults[id]; ok {
				r2 := result
				rp = &r2
			}
			return &item, rp, true
		}
	}
	return nil, nil, false
}
