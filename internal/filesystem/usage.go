package filesystem

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"
)

type Usage struct {
	Input       int `json:"input"`
	Output      int `json:"output"`
	CacheCreate int `json:"cache_create,omitempty"`
	CacheRead   int `json:"cache_read,omitempty"`
}

func UpdateUsage(model string, input, output, cacheCreate, cacheRead int) error {
	if model == "" {
		return nil
	}

	lockPath := UsagePath + ".lock"
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
	if bytes, err := os.ReadFile(UsagePath); err == nil {
		_ = json.Unmarshal(bytes, &usageMap)
	}

	prev := usageMap[model]
	usageMap[model] = Usage{
		Input:       prev.Input + input,
		Output:      prev.Output + output,
		CacheCreate: prev.CacheCreate + cacheCreate,
		CacheRead:   prev.CacheRead + cacheRead,
	}

	bytes, err := json.Marshal(usageMap)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	if err := go_utils_filesystem.WriteFile(UsagePath, string(bytes), 0644); err != nil {
		return fmt.Errorf("go_utils_filesystem.WriteFile: %w", err)
	}

	return nil
}
