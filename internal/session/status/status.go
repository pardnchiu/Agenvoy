package sessionStatus

import (
	"log/slog"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func Get(sessionID string) Status {
	if sessionID == "" {
		return Status{}
	}
	mu.Lock()
	defer mu.Unlock()
	return get(sessionID)
}

func Clear() {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem/reader ListDirs",
			slog.String("dir", filesystem.SessionsDir),
			slog.String("error", err.Error()))
		return
	}
	for _, dir := range dirs {
		clear(dir.Name)
	}
}

func clear(sessionID string) {
	if sessionID == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	status := get(sessionID)
	if len(status.Active) == 0 && status.State != StatusOnline {
		return
	}
	status.Active = nil
	status.State = StatusIdle
	status.EndedAt = time.Now().Format("2006-01-02 15:04:05.000")
	write(sessionID, status)
}
