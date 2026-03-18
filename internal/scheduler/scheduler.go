package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/sandbox"
	goCron "github.com/pardnchiu/go-scheduler"
)

type cronEngine interface {
	Start()
	Stop() context.Context
	Add(spec string, action any, arg ...any) (int64, error)
	Remove(id int64)
}

type Scheduler struct {
	mu          sync.Mutex
	timers      map[string]*time.Timer
	tasks       []filesystem.TaskItem
	crons       []filesystem.CronItem
	cron        cronEngine
	OnCompleted OnCompletedFn
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
			timers: make(map[string]*time.Timer),
			cron:   c,
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

	s.mu.Lock()
	for _, timer := range s.timers {
		timer.Stop()
	}
	s.mu.Unlock()

	s.cron.Stop()
}

func newID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|") + fmt.Sprint(time.Now().UnixNano())))
	return hex.EncodeToString(h[:])[:8]
}

func runScript(caller, scriptPath string) string {
	var binary string
	switch strings.ToLower(filepath.Ext(scriptPath)) {
	case ".py":
		binary = "python3"
	default:
		binary = "sh"
	}

	workDir := filepath.Dir(scriptPath)
	wrappedBin, wrappedArgs, err := sandbox.Wrap(binary, []string{scriptPath}, workDir)
	if err != nil {
		slog.Error(caller,
			slog.String("script", filepath.Base(scriptPath)),
			slog.String("error", err.Error()))
		return fmt.Sprintf("error: %s", err.Error())
	}

	cmd := exec.Command(wrappedBin, wrappedArgs...)
	cmd.Env = append(os.Environ(),
		"PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/opt/homebrew/sbin",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		output := strings.TrimSpace(string(out))
		slog.Error(caller,
			slog.String("script", filepath.Base(scriptPath)),
			slog.String("error", err.Error()),
			slog.String("output", output))
		if output != "" {
			return fmt.Sprintf("error: %s\n%s", err.Error(), output)
		}
		return fmt.Sprintf("error: %s", err.Error())
	}
	return strings.TrimSpace(string(out))
}

func removeScript(scriptPath string) {
	if err := os.Remove(scriptPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("os.Remove",
			slog.String("script", scriptPath),
			slog.String("error", err.Error()))
	}
}
