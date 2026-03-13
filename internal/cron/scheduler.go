package cron

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var scheduler *Scheduler

func Get() *Scheduler {
	return scheduler
}

func Stop() {
	if scheduler != nil {
		scheduler.stop()
	}
}

type OnCompletedFn func(channelID, output string)

type Scheduler struct {
	mu          sync.Mutex
	timers      map[string]*time.Timer
	OnCompleted OnCompletedFn
}

type taskItem struct {
	line      string
	at        time.Time
	script    string
	channelID string
}

func New() error {
	scheduler = &Scheduler{
		timers: make(map[string]*time.Timer),
	}
	return nil
}

func (s *Scheduler) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines, err := filesystem.ReadFile(filesystem.TasksPath)
	if err != nil {
		return err
	}

	now := time.Now()
	var skip []string

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			skip = append(skip, line)
			continue
		}

		item, err := parseLine(trim)
		if err != nil {
			// * cannot parse, then skip
			slog.Warn("parseLine",
				slog.String("error", err.Error()))
			continue
		}

		if !item.at.After(now) {
			// * already gone, then skip
			continue
		}

		skip = append(skip, trim)
		s.setTask(item)
	}

	return filesystem.WriteFile(filesystem.TasksPath, linesToContent(skip), 0644)
}

func parseLine(line string) (taskItem, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return taskItem{}, fmt.Errorf("at least 2 fields `{time} {script}`")
	}

	at, err := time.Parse(time.RFC3339, fields[0])
	if err != nil {
		return taskItem{}, fmt.Errorf("not RFC3339: %w", err)
	}

	item := taskItem{
		line:   line,
		at:     at,
		script: fields[1],
	}
	if len(fields) >= 3 {
		item.channelID = fields[2]
	}
	return item, nil
}

func linesToContent(lines []string) string {
	newContent := strings.Join(lines, "\n")
	if len(lines) > 0 {
		newContent += "\n"
	}
	return newContent
}

func (s *Scheduler) setTask(item taskItem) error {
	if err := os.MkdirAll(filesystem.ScriptsDir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}

	scriptPath := filepath.Join(filesystem.ScriptsDir, item.script)
	delay := time.Until(item.at)

	execTime := time.AfterFunc(delay, func() {
		output, err := runScript(scriptPath)
		if err != nil {
			slog.Error("runScript",
				slog.String("error", err.Error()))
			output = fmt.Sprintf("error: %s", err.Error())
		}

		if item.channelID != "" {
			s.mu.Lock()
			cb := s.OnCompleted
			s.mu.Unlock()
			if cb != nil {
				cb(item.channelID, output)
			}
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		delete(s.timers, item.line)
		removeLine(filesystem.TasksPath, item.line)
	})
	s.timers[item.line] = execTime
	return nil
}

func runScript(scriptPath string) (string, error) {
	var cmd *exec.Cmd
	switch strings.ToLower(filepath.Ext(scriptPath)) {
	case ".py":
		cmd = exec.Command("python3", scriptPath)
	default:
		cmd = exec.Command("sh", scriptPath)
	}

	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (s *Scheduler) AddTask(at time.Time, script, channelID string) error {
	if !at.After(time.Now()) {
		return fmt.Errorf("already gone")
	}

	line := fmt.Sprintf("%s %s", at.UTC().Format(time.RFC3339), script)
	if channelID != "" {
		line = fmt.Sprintf("%s %s", line, channelID)
	}
	item, _ := parseLine(line)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := appendLine(filesystem.TasksPath, line); err != nil {
		return fmt.Errorf("appendLine: %w", err)
	}

	return s.setTask(item)
}

func (s *Scheduler) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.timers {
		t.Stop()
	}
}

func appendLine(path, line string) error {
	lines, err := filesystem.ReadFile(path)
	if err != nil {
		return err
	}
	return filesystem.WriteFile(path, linesToContent(append(lines, line)), 0644)
}

func removeLine(path, target string) {
	lines, err := filesystem.ReadFile(path)
	if err != nil {
		return
	}
	var kept []string
	for _, l := range lines {
		if strings.TrimSpace(l) != target {
			kept = append(kept, l)
		}
	}
	filesystem.WriteFile(path, linesToContent(kept), 0644)
}
