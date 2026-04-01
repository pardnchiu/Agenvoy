package filesystem

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
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
	data, err := os.ReadFile(TasksPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var items []TaskItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return items, nil
}

func WriteTasks(items []TaskItem) error {
	data, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	return WriteFile(TasksPath, string(data), 0644)
}

func GetCrons() ([]CronItem, error) {
	data, err := os.ReadFile(CronsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var items []CronItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return items, nil
}

func WriteCrons(crons []CronItem) error {
	bytes, err := json.Marshal(crons)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	if err := WriteFile(CronsPath, string(bytes), 0644); err != nil {
		return fmt.Errorf("WriteFile: %w", err)
	}
	return nil
}

func WriteTaskResult(r TaskResult) error {
	bytes, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	return WriteFile(TaskResultPath(r.ID, r.At), string(bytes), 0644)
}

func DeleteTaskResult(id string, at time.Time) error {
	err := os.Remove(TaskResultPath(id, at))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func GetAllTaskResults() ([]TaskResult, error) {
	matches, err := filepath.Glob(filepath.Join(SchedulerStateDir, "task_*.json"))
	if err != nil {
		return nil, fmt.Errorf("filepath.Glob: %w", err)
	}
	results := make([]TaskResult, 0, len(matches))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var result TaskResult
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}
		results = append(results, result)
	}
	return results, nil
}

func WriteCronResult(r CronResult) error {
	bytes, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	return WriteFile(CronResultPath(r.ID), string(bytes), 0644)
}

func WriteCronRecord(result CronResult) error {
	if result.RunAt == nil {
		return nil
	}
	bytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	return WriteFile(CronRecordPath(result.ID, *result.RunAt), string(bytes), 0644)
}

func DeleteCronResult(id string) error {
	err := os.Remove(CronResultPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func GetAllCronResults() ([]CronResult, error) {
	matches, err := filepath.Glob(filepath.Join(SchedulerStateDir, "cron_*.json"))
	if err != nil {
		return nil, fmt.Errorf("filepath.Glob: %w", err)
	}
	results := make([]CronResult, 0, len(matches))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var result CronResult
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}
		results = append(results, result)
	}
	return results, nil
}
