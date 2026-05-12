package runtime

import (
	"fmt"
	"path/filepath"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type TaskEntry struct {
	At        time.Time `json:"at"`
	SessionID string    `json:"session_id"`
	Skill     string    `json:"skill"`
}

type CronEntry struct {
	Expression string `json:"expression"`
	SessionID  string `json:"session_id"`
	Skill      string `json:"skill"`
}

func TaskKey(t TaskEntry) string {
	return fmt.Sprintf("%d|%s|%s", t.At.UnixNano(), t.SessionID, t.Skill)
}

func CronKey(c CronEntry) string {
	return fmt.Sprintf("%s|%s|%s", c.Expression, c.SessionID, c.Skill)
}

func LoadTasks() ([]TaskEntry, error) {
	if !go_pkg_filesystem_reader.Exists(filesystem.TasksPath) {
		return nil, nil
	}
	return go_pkg_filesystem.ReadJSON[[]TaskEntry](filesystem.TasksPath)
}

func SaveTasks(tasks []TaskEntry) error {
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(filesystem.TasksPath), true); err != nil {
		return fmt.Errorf("CheckDir: %w", err)
	}
	if tasks == nil {
		tasks = []TaskEntry{}
	}
	return go_pkg_filesystem.WriteJSON(filesystem.TasksPath, tasks, true)
}

func LoadCrons() ([]CronEntry, error) {
	if !go_pkg_filesystem_reader.Exists(filesystem.CronsPath) {
		return nil, nil
	}
	return go_pkg_filesystem.ReadJSON[[]CronEntry](filesystem.CronsPath)
}

func SaveCrons(crons []CronEntry) error {
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(filesystem.CronsPath), true); err != nil {
		return fmt.Errorf("CheckDir: %w", err)
	}
	if crons == nil {
		crons = []CronEntry{}
	}
	return go_pkg_filesystem.WriteJSON(filesystem.CronsPath, crons, true)
}

func AppendTask(t TaskEntry) error {
	existing, _ := LoadTasks()
	key := TaskKey(t)
	for _, e := range existing {
		if TaskKey(e) == key {
			return fmt.Errorf("duplicate task: %s already scheduled at the same time", t.Skill)
		}
	}
	existing = append(existing, t)
	return SaveTasks(existing)
}

func AppendCron(c CronEntry) error {
	existing, _ := LoadCrons()
	key := CronKey(c)
	for _, e := range existing {
		if CronKey(e) == key {
			return fmt.Errorf("duplicate cron: %s already scheduled with the same expression", c.Skill)
		}
	}
	existing = append(existing, c)
	return SaveCrons(existing)
}

func RemoveTask(t TaskEntry) (bool, error) {
	existing, err := LoadTasks()
	if err != nil {
		return false, err
	}
	key := TaskKey(t)
	filtered := make([]TaskEntry, 0, len(existing))
	found := false
	for _, e := range existing {
		if TaskKey(e) == key {
			found = true
			continue
		}
		filtered = append(filtered, e)
	}
	if !found {
		return false, nil
	}
	return true, SaveTasks(filtered)
}
