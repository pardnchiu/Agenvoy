package record

import (
	"fmt"
	"os"
	"syscall"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
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

	lockPath := filesystem.UsagePath + ".lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("os.OpenFile [%s]: %w", lockPath, err)
	}
	defer file.Close()

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("syscall.Flock [%s]: %w", lockPath, err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	usage := make(map[string]Usage)
	if go_pkg_filesystem_reader.Exists(filesystem.UsagePath) {
		if loaded, err := go_pkg_filesystem.ReadJSON[map[string]Usage](filesystem.UsagePath); err == nil && loaded != nil {
			usage = loaded
		}
	}

	prev := usage[model]
	usage[model] = Usage{
		Input:       prev.Input + input,
		Output:      prev.Output + output,
		CacheCreate: prev.CacheCreate + cacheCreate,
		CacheRead:   prev.CacheRead + cacheRead,
	}

	if err := go_pkg_filesystem.WriteJSON(filesystem.UsagePath, usage, false); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem.WriteJSON [%s]: %w", filesystem.UsagePath, err)
	}
	return nil
}
