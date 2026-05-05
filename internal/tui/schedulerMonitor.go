package tui

import (
	"log/slog"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
)

func SchedulerMonitor() {
	if err := go_pkg_filesystem.CheckDir(filesystem.SchedulerDir, true); err != nil {
		slog.Warn("go_pkg_filesystem.CheckDir",
			slog.String("error", err.Error()))
		return
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("fsnotify.NewWatcher",
			slog.String("error", err.Error()))
		return
	}
	defer watcher.Close()

	if err := watcher.Add(filesystem.SchedulerDir); err != nil {
		slog.Warn("watcher.Add",
			slog.String("dir", filesystem.SchedulerDir),
			slog.String("error", err.Error()))
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if !(event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)) {
				continue
			}
			changedBase := filepath.Base(event.Name)
			if changedBase != "tasks.json" && changedBase != "crons.json" {
				continue
			}
			s := scheduler.Get()
			if s == nil {
				continue
			}
			s.Reset()
			if err := tasks.Setup(s); err != nil {
				slog.Warn("tasks.Setup",
					slog.String("error", err.Error()))
			}
			if err := crons.Setup(s); err != nil {
				slog.Warn("crons.Setup",
					slog.String("error", err.Error()))
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("SchedulerMonitor",
				slog.String("error", err.Error()))
		}
	}
}
