package filesystem

import (
	"fmt"
	"os"
	"syscall"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
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

	// * lock file: kept on os.OpenFile because syscall.Flock needs the raw fd
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
	if go_pkg_filesystem_reader.Exists(UsagePath) {
		if loaded, err := go_pkg_filesystem.ReadJSON[map[string]Usage](UsagePath); err == nil && loaded != nil {
			usageMap = loaded
		}
	}

	prev := usageMap[model]
	usageMap[model] = Usage{
		Input:       prev.Input + input,
		Output:      prev.Output + output,
		CacheCreate: prev.CacheCreate + cacheCreate,
		CacheRead:   prev.CacheRead + cacheRead,
	}

	if err := go_pkg_filesystem.WriteJSON(UsagePath, usageMap, false); err != nil {
		return fmt.Errorf("go_pkg_filesystem.WriteJSON: %w", err)
	}

	return nil
}
