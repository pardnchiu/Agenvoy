package scheduler

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (s *Scheduler) LoadCrons() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines, err := filesystem.ReadFileSlice(filesystem.CronsPath)
	if err != nil {
		return fmt.Errorf("filesystem.ReadFile: %w", err)
	}

	var tasks []cronItem
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}

		item, err := parseCronLine(trim)
		if err != nil {
			slog.Warn("parseCronLine",
				slog.String("error", err.Error()))
			continue
		}

		id, err := s.cron.Add(item.expression, s.makeCronAction(item))
		if err != nil {
			slog.Warn("s.cron.Add",
				slog.String("error", err.Error()))
			continue
		}

		item.cronID = id
		tasks = append(tasks, item)
	}

	s.crons = tasks
	return nil
}

func (s *Scheduler) RemoveCronTask(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 1 || index > len(s.crons) {
		return fmt.Errorf("not exist")
	}

	target := s.crons[index-1]

	s.cron.Remove(target.cronID)
	removeLine(filesystem.CronsPath, target.line)
	s.crons = append(s.crons[:index-1], s.crons[index:]...)
	removeScript(filepath.Join(filesystem.ScriptsDir, target.script))
	return nil
}

func (s *Scheduler) ListCronTasks() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]string, len(s.crons))
	for i, t := range s.crons {
		result[i] = fmt.Sprintf("%d. %s", i+1, t.line)
	}
	return result
}
