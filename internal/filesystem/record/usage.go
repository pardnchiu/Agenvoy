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

	path := filesystem.UsagePath
	lockPath := path + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("os OpenFile [%x]: %w", lockPath, err)
	}
	defer lock.Close()

	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("syscall Flock: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	usage := make(map[string]Usage)
	if go_pkg_filesystem_reader.Exists(path) {
		if loaded, err := go_pkg_filesystem.ReadJSON[map[string]Usage](path); err == nil && loaded != nil {
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

	if err := go_pkg_filesystem.WriteJSON(path, usage, false); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteJSON [%s]: %w", path, err)
	}
	return nil
}
