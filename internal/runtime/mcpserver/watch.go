package mcpserver

import (
	"context"
	"log/slog"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
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
		filesystem.APIToolsDir,
		filesystem.WorkAPIToolsDir,
		filesystem.ExtensionScriptToolsDir,
		filesystem.ExtensionAPIToolsDir,
	}
	for _, dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			slog.Warn("fsnotify.Add",
				slog.String("dir", dir),
				slog.String("error", err.Error()))
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
				slog.Warn("fsnotify.Error",
					slog.String("error", err.Error()))
			}
		}
	}()
}
