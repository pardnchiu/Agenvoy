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

func TaskKey(t TaskEntry) string {
	return fmt.Sprintf("%d|%s|%s", t.At.UnixNano(), t.SessionID, t.Skill)
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

func RemoveTask(name string) (int, error) {
	existing, err := LoadTasks()
	if err != nil {
		return 0, err
	}
	filtered := make([]TaskEntry, 0, len(existing))
	removed := 0
	for _, e := range existing {
		if e.Skill == name {
			removed++
			continue
		}
		filtered = append(filtered, e)
	}
	if removed == 0 {
		return 0, nil
	}
	return removed, SaveTasks(filtered)
}

func RemoveTaskByTimeSkill(at time.Time, skill string) (int, error) {
	existing, err := LoadTasks()
	if err != nil {
		return 0, err
	}
	target := at.UnixNano()
	filtered := make([]TaskEntry, 0, len(existing))
	removed := 0
	for _, e := range existing {
		if e.Skill == skill && e.At.UnixNano() == target {
			removed++
			continue
		}
		filtered = append(filtered, e)
	}
	if removed == 0 {
		return 0, nil
	}
	return removed, SaveTasks(filtered)
}

func HasTaskForSkill(skill string) (bool, error) {
	existing, err := LoadTasks()
	if err != nil {
		return false, err
	}
	for _, e := range existing {
		if e.Skill == skill {
			return true, nil
		}
	}
	return false, nil
}

func PatchTask(skillName string, newAt time.Time) (int, error) {
	existing, err := LoadTasks()
	if err != nil {
		return 0, err
	}
	patched := 0
	for i := range existing {
		if existing[i].Skill != skillName {
			continue
		}
		existing[i].At = newAt
		patched++
	}
	if patched == 0 {
		return 0, nil
	}
	return patched, SaveTasks(existing)
}
