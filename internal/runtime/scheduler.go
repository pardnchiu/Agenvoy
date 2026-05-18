package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	goCron "github.com/pardnchiu/go-scheduler"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type Runner func(ctx context.Context, sessionID, skillName string) (string, error)

var runnerFn atomic.Pointer[Runner]

func SetRunner(r Runner) {
	runnerFn.Store(&r)
}

type state struct {
	mu      sync.Mutex
	cron    schedulerCron
	timers  map[string]*time.Timer
	cronIDs map[string]int64
}

type schedulerCron interface {
	Start()
	Stop() context.Context
	Add(spec string, action any, arg ...any) (int64, error)
	Remove(id int64)
}

var (
	s    *state
	once sync.Once
	smu  sync.RWMutex
)

func NewScheduler() error {
	var initErr error
	once.Do(func() {
		c, err := goCron.New(goCron.Config{})
		if err != nil {
			initErr = fmt.Errorf("goCron.New: %w", err)
			return
		}
		c.Start()
		smu.Lock()
		s = &state{
			cron:    c,
			timers:  make(map[string]*time.Timer),
			cronIDs: make(map[string]int64),
		}
		smu.Unlock()
	})
	if initErr != nil {
		return initErr
	}
	if err := reload(); err != nil {
		slog.Warn("scheduler initial reload",
			slog.String("error", err.Error()))
	}
	return nil
}

func StopScheduler() {
	smu.RLock()
	cur := s
	smu.RUnlock()
	if cur == nil {
		return
	}
	cur.mu.Lock()
	for _, t := range cur.timers {
		t.Stop()
	}
	for _, id := range cur.cronIDs {
		cur.cron.Remove(id)
	}
	cur.timers = make(map[string]*time.Timer)
	cur.cronIDs = make(map[string]int64)
	cur.mu.Unlock()
	cur.cron.Stop()
}

func get() *state {
	smu.RLock()
	defer smu.RUnlock()
	return s
}

func reload() error {
	st := get()
	if st == nil {
		return fmt.Errorf("scheduler not initialized")
	}

	tasks, err := LoadTasks()
	if err != nil {
		return fmt.Errorf("LoadTasks: %w", err)
	}
	crons, err := LoadCrons()
	if err != nil {
		return fmt.Errorf("LoadCrons: %w", err)
	}

	now := time.Now()
	diskTaskKeys := make(map[string]TaskEntry, len(tasks))
	for _, t := range tasks {
		diskTaskKeys[TaskKey(t)] = t
	}
	diskCronKeys := make(map[string]CronEntry, len(crons))
	for _, c := range crons {
		diskCronKeys[CronKey(c)] = c
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	for key, timer := range st.timers {
		if _, keep := diskTaskKeys[key]; !keep {
			timer.Stop()
			delete(st.timers, key)
		}
	}
	for key, id := range st.cronIDs {
		if _, keep := diskCronKeys[key]; !keep {
			st.cron.Remove(id)
			delete(st.cronIDs, key)
		}
	}

	for key, entry := range diskTaskKeys {
		if _, exists := st.timers[key]; exists {
			continue
		}
		if !entry.At.After(now) {
			go func(e TaskEntry) {
				if _, err := RemoveTaskByTimeSkill(e.At, e.Skill); err != nil {
					slog.Warn("RemoveTaskByTimeSkill",
						slog.String("error", err.Error()))
				}
			}(entry)
			continue
		}
		entryCopy := entry
		keyCopy := key
		delay := time.Until(entry.At)
		timer := time.AfterFunc(delay, func() {
			fire(entryCopy.SessionID, entryCopy.Skill)
			st.mu.Lock()
			delete(st.timers, keyCopy)
			st.mu.Unlock()
			if _, err := RemoveTaskByTimeSkill(entryCopy.At, entryCopy.Skill); err != nil {
				slog.Warn("RemoveTaskByTimeSkill",
					slog.String("error", err.Error()))
				return
			}
			hasMore, err := HasTaskForSkill(entryCopy.Skill)
			if err != nil {
				slog.Warn("HasTaskForSkill",
					slog.String("error", err.Error()))
				return
			}
			if hasMore {
				return
			}
			if err := filesystem.TrashScheduleSkill(entryCopy.Skill); err != nil {
				slog.Warn("filesystem.TrashScheduleSkill",
					slog.String("error", err.Error()))
			}
		})
		st.timers[keyCopy] = timer
	}

	for key, entry := range diskCronKeys {
		if _, exists := st.cronIDs[key]; exists {
			continue
		}
		entryCopy := entry
		id, err := st.cron.Add(entry.Expression, func() {
			fire(entryCopy.SessionID, entryCopy.Skill)
		})
		if err != nil {
			slog.Warn("cron.Add",
				slog.String("error", err.Error()))
			continue
		}
		st.cronIDs[key] = id
	}

	return nil
}

func fire(sessionID, skillName string) {
	fn := runnerFn.Load()
	if fn == nil || *fn == nil {
		return
	}
	ctx := context.Background()
	if _, err := (*fn)(ctx, sessionID, skillName); err != nil {
		slog.Warn("scheduler.fire: runner error",
			slog.String("error", err.Error()))
	}
}

func SchedulerWatcher(ctx context.Context) func() {
	watchDir := filepath.Dir(filesystem.TasksPath)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("SchedulerWatcher.NewWatcher",
			slog.String("error", err.Error()))
		return func() {}
	}
	if err := w.Add(watchDir); err != nil {
		slog.Warn("SchedulerWatcher.Add",
			slog.String("dir", watchDir),
			slog.String("error", err.Error()))
		_ = w.Close()
		return func() {}
	}

	stopCh := make(chan struct{})
	go func() {
		defer w.Close()
		var lastReload time.Time
		for {
			select {
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				base := filepath.Base(ev.Name)
				if base != "tasks.json" && base != "crons.json" {
					continue
				}
				if !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Rename) {
					continue
				}
				if time.Since(lastReload) < 200*time.Millisecond {
					continue
				}
				lastReload = time.Now()
				if err := reload(); err != nil {
					slog.Warn("ReloadScheduler",
						slog.String("error", err.Error()))
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				slog.Warn("SchedulerWatcher",
					slog.String("error", err.Error()))
			}
		}
	}()
	return func() { close(stopCh) }
}
