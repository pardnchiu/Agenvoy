package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	goCron "github.com/pardnchiu/go-scheduler"
)

type Scheduler struct {
	Mu          sync.Mutex
	Timers      map[string]*time.Timer
	Tasks       []filesystem.TaskItem
	TaskResults map[string]filesystem.TaskResult
	Crons       []filesystem.CronItem
	CronResults map[string]filesystem.CronResult
	Cron        schedulerCron
	OnCompleted OnCompletedFn
}

type schedulerCron interface {
	Start()
	Stop() context.Context
	Add(spec string, action any, arg ...any) (int64, error)
	Remove(id int64)
}

type OnCompletedFn func(channelID, output string)

var (
	scheduler *Scheduler
	once      sync.Once
	mu        sync.RWMutex
)

func New() error {
	var initErr error
	once.Do(func() {
		c, err := goCron.New(goCron.Config{})
		if err != nil {
			initErr = err
			return
		}
		c.Start()
		mu.Lock()
		scheduler = &Scheduler{
			Timers:      make(map[string]*time.Timer),
			TaskResults: make(map[string]filesystem.TaskResult),
			CronResults: make(map[string]filesystem.CronResult),
			Cron:        c,
		}
		mu.Unlock()
	})
	return initErr
}

func Get() *Scheduler {
	mu.RLock()
	defer mu.RUnlock()
	return scheduler
}

func Stop() {
	mu.RLock()
	s := scheduler
	mu.RUnlock()

	if s == nil {
		return
	}

	s.Mu.Lock()
	for _, timer := range s.Timers {
		timer.Stop()
	}
	s.Mu.Unlock()

	s.Cron.Stop()
}
