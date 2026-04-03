package crons

import (
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
)

func GetCron(s *scheduler.Scheduler, id string) (*filesystem.CronItem, *filesystem.CronResult, bool) {
	if s == nil {
		items, err := filesystem.GetCrons()
		if err != nil {
			return nil, nil, false
		}
		var item *filesystem.CronItem
		for _, c := range items {
			if c.ID == id {
				cp := c
				item = &cp
				break
			}
		}
		if item == nil {
			return nil, nil, false
		}
		results, err := filesystem.GetAllCronResults()
		if err != nil {
			return item, nil, true
		}
		for _, r := range results {
			if r.ID == id {
				cp := r
				return item, &cp, true
			}
		}
		return item, nil, true
	}

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
