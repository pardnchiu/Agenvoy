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
	"github.com/pardnchiu/agenvoy/internal/utils"
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

type Scheduler struct {
	mu           sync.Mutex
	schedulerDir string
	scriptsDir   string
	timers       map[string]*time.Timer
}

type taskItem struct {
	line   string
	at     time.Time
	script string
}

func New() error {
	// * ~/.config/agenvoy/scheduler
	schedulerDir, err := utils.GetConfigDir("scheduler")
	if err != nil {
		return fmt.Errorf("utils.GetConfigDir: %w", err)
	}

	// * ~/.config/agenvoy/scheduler/scripts
	scriptsDir := filepath.Join(schedulerDir.Home, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}

	scheduler = &Scheduler{
		schedulerDir: schedulerDir.Home,
		scriptsDir:   scriptsDir,
		timers:       make(map[string]*time.Timer),
	}
	return nil
}

func (s *Scheduler) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines, err := filesystem.ReadFile(s.schedulerDir)
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

	return filesystem.WriteFile(s.schedulerDir, linesToContent(skip), 0644)
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
	return taskItem{
		line:   line,
		at:     at,
		script: strings.Join(fields[1:], " "),
	}, nil
}

func linesToContent(lines []string) string {
	newContent := strings.Join(lines, "\n")
	if len(lines) > 0 {
		newContent += "\n"
	}
	return newContent
}

func (s *Scheduler) setTask(item taskItem) {
	scriptPath := filepath.Join(s.scriptsDir, item.script)
	delay := time.Until(item.at)

	execTime := time.AfterFunc(delay, func() {
		if err := runScript(scriptPath); err != nil {
			slog.Error("runScript",
				slog.String("error", err.Error()))
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		delete(s.timers, item.line)
		removeLine(s.schedulerDir, item.line)
	})
	s.timers[item.line] = execTime
}

func runScript(scriptPath string) error {
	var cmd *exec.Cmd
	switch strings.ToLower(filepath.Ext(scriptPath)) {
	case ".py":
		cmd = exec.Command("python3", scriptPath)
	default:
		cmd = exec.Command("sh", scriptPath)
	}

	_, err := cmd.CombinedOutput()
	return err
}

func (s *Scheduler) AddTask(at time.Time, script string) error {
	if !at.After(time.Now()) {
		return fmt.Errorf("already gone")
	}

	line := fmt.Sprintf("%s %s", at.UTC().Format(time.RFC3339), script)
	item, _ := parseLine(line)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := appendLine(s.schedulerDir, line); err != nil {
		return fmt.Errorf("appendLine: %w", err)
	}
	s.setTask(item)
	return nil
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
