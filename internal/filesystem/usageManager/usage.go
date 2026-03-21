package usageManager

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type Usage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

func Update(model string, input, output int) error {
	if model == "" || (input == 0 && output == 0) {
		return nil
	}

	lockPath := filesystem.UsagePath + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("os.OpenFile: %w", err)
	}
	defer lock.Close()

	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("syscall.Flock: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	usageMap := make(map[string]Usage)
	if bytes, err := os.ReadFile(filesystem.UsagePath); err == nil {
		_ = json.Unmarshal(bytes, &usageMap)
	}

	prev := usageMap[model]
	usageMap[model] = Usage{
		Input:  prev.Input + input,
		Output: prev.Output + output,
	}

	bytes, err := json.Marshal(usageMap)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	if err := filesystem.WriteFile(filesystem.UsagePath, string(bytes), 0644); err != nil {
		return fmt.Errorf("filesystem.WriteFile: %w", err)
	}

	return nil
}
