package filesystem

import (
	"fmt"
	"path/filepath"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func TaskResultPath(id string, at time.Time) string {
	return filepath.Join(SchedulerStateDir, fmt.Sprintf("task_%s_%s.json", id, at.Local().Format("20060102_1504")))
}

func CronResultPath(id string) string {
	return filepath.Join(SchedulerStateDir, fmt.Sprintf("cron_%s.json", id))
}

func CronRecordPath(id string, at time.Time) string {
	return filepath.Join(SchedulerRecordsDir, fmt.Sprintf("cron_%s_%s.json", id, at.Local().Format("20060102_150405")))
}

type TaskItem struct {
	ID        string    `json:"id"`
	At        time.Time `json:"at"`
	Script    string    `json:"script"`
	ChannelID string    `json:"channel_id,omitempty"`
}

type CronItem struct {
	ID         string `json:"id"`
	Expression string `json:"expression"`
	Script     string `json:"script"`
	ChannelID  string `json:"channel_id,omitempty"`
	CronID     int64  `json:"-"`
}

type TaskResult struct {
	ID         string     `json:"id"`
	At         time.Time  `json:"at"`
	Script     string     `json:"script"`
	Status     string     `json:"status"` // running | completed | failed
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Output     string     `json:"output,omitempty"`
	Err        string     `json:"err,omitempty"`
}

type CronResult struct {
	ID     string     `json:"id"`
	RunAt  *time.Time `json:"run_at,omitempty"`
	Status string     `json:"status"` // pending | completed | failed
	Output string     `json:"output,omitempty"`
	Err    string     `json:"err,omitempty"`
}

func GetTasks() ([]TaskItem, error) {
	if !go_pkg_filesystem_reader.Exists(TasksPath) {
		return nil, nil
	}
	items, err := go_pkg_filesystem.ReadJSON[[]TaskItem](TasksPath)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func WriteTasks(items []TaskItem) error {
	return go_pkg_filesystem.WriteJSON(TasksPath, items, false)
}

func GetCrons() ([]CronItem, error) {
	if !go_pkg_filesystem_reader.Exists(CronsPath) {
		return nil, nil
	}
	items, err := go_pkg_filesystem.ReadJSON[[]CronItem](CronsPath)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func WriteCrons(crons []CronItem) error {
	if err := go_pkg_filesystem.WriteJSON(CronsPath, crons, false); err != nil {
		return fmt.Errorf("WriteJSON: %w", err)
	}
	return nil
}

func WriteTaskResult(r TaskResult) error {
	return go_pkg_filesystem.WriteJSON(TaskResultPath(r.ID, r.At), r, false)
}

func DeleteTaskResult(id string, at time.Time) error {
	return go_pkg_filesystem.Remove(TaskResultPath(id, at))
}

func GetAllTaskResults() ([]TaskResult, error) {
	matches, err := filepath.Glob(filepath.Join(SchedulerStateDir, "task_*.json"))
	if err != nil {
		return nil, fmt.Errorf("filepath.Glob: %w", err)
	}
	results := make([]TaskResult, 0, len(matches))
	for _, path := range matches {
		result, err := go_pkg_filesystem.ReadJSON[TaskResult](path)
		if err != nil {
			continue
		}
		results = append(results, result)
	}
	return results, nil
}

func WriteCronResult(r CronResult) error {
	return go_pkg_filesystem.WriteJSON(CronResultPath(r.ID), r, false)
}

func WriteCronRecord(result CronResult) error {
	if result.RunAt == nil {
		return nil
	}
	return go_pkg_filesystem.WriteJSON(CronRecordPath(result.ID, *result.RunAt), result, false)
}

func DeleteCronResult(id string) error {
	return go_pkg_filesystem.Remove(CronResultPath(id))
}

func GetAllCronResults() ([]CronResult, error) {
	matches, err := filepath.Glob(filepath.Join(SchedulerStateDir, "cron_*.json"))
	if err != nil {
		return nil, fmt.Errorf("filepath.Glob: %w", err)
	}
	results := make([]CronResult, 0, len(matches))
	for _, path := range matches {
		result, err := go_pkg_filesystem.ReadJSON[CronResult](path)
		if err != nil {
			continue
		}
		results = append(results, result)
	}
	return results, nil
}
