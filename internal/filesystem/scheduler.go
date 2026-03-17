package filesystem

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

type CronItem struct {
	ID         string `json:"id"`
	Expression string `json:"expression"`
	Script     string `json:"script"`
	ChannelID  string `json:"channel_id,omitempty"`
	CronID     int64
}

func GetCrons() ([]CronItem, error) {
	data, err := ReadFile(CronsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var items []CronItem
	if err := json.Unmarshal([]byte(data), &items); err != nil {
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

type TaskItem struct {
	ID        string    `json:"id"`
	At        time.Time `json:"at"`
	Script    string    `json:"script"`
	ChannelID string    `json:"channel_id,omitempty"`
}

func GetTasks() ([]TaskItem, error) {
	data, err := ReadFile(TasksPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var items []TaskItem
	if err := json.Unmarshal([]byte(data), &items); err != nil {
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
