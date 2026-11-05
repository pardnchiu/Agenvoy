package mcp

import (
	"context"
	"log/slog"
	"time"

	"github.com/fsnotify/fsnotify"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func addWatch(watcher *fsnotify.Watcher, dir string) {
	if err := watcher.Add(dir); err != nil {
		slog.Debug("fsnotify.Add",
			slog.String("dir", dir),
			slog.String("error", err.Error()))
	}
}

func listToolDirs(dir string) []string {
	entries, err := go_pkg_filesystem_reader.ListDirs(dir)
	if err != nil {
		return nil
	}
	list := make([]string, 0, len(entries))
	for _, entry := range entries {
		list = append(list, entry.Path)
	}
	return list
}

func (s *Server) notify() {
	s.write(notification{
		JSONRPC: "2.0",
		Method:  "notifications/tools/list_changed",
	})
}

func (s *Server) watch(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("fsnotify.NewWatcher",
			slog.String("error", err.Error()))
		return
	}

	dirs := []string{
		filesystem.ScriptToolsDir,
		filesystem.WorkScriptToolsDir,
		filesystem.ExtensionScriptToolsDir,
	}
	for _, dir := range dirs {
		addWatch(watcher, dir)
		for _, sub := range listToolDirs(dir) {
			addWatch(watcher, sub)
		}
	}

	go func() {
		defer watcher.Close()

		var debounce *time.Timer

		for {
			select {
			case <-ctx.Done():
				if debounce != nil {
					debounce.Stop()
				}
				return

			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				if !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Remove) && !ev.Has(fsnotify.Rename) {
					continue
				}
				if ev.Has(fsnotify.Create) && go_pkg_filesystem_reader.IsDir(ev.Name) {
					addWatch(watcher, ev.Name)
				}

				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.AfterFunc(500*time.Millisecond, func() {
					toolBox := scanTools()

					s.readMu.Lock()
					s.toolBox = toolBox
					s.readMu.Unlock()

					s.notify()
				})

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Debug("fsnotify.Error",
					slog.String("error", err.Error()))
			}
		}
	}()
}
