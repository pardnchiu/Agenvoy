package tui

import (
	"log/slog"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func FileMonitor() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("fsnotify.NewWatcher",
			slog.String("error", err.Error()))
		return
	}
	defer watcher.Close()

	if err := watcher.Add(filesystem.AgenvoyDir); err != nil {
		slog.Warn("watcher.Add", slog.String("dir", filesystem.AgenvoyDir),
			slog.String("error", err.Error()))
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				changedBase := filepath.Base(event.Name)
				go app.QueueUpdateDraw(func() {
					loadDir(filesView, currentDir)
					if currentPath == "" {
						// Default content shown: refresh if config.json or usage.json changed
						if changedBase == "config.json" || changedBase == "usage.json" {
							contentView.SetText(setDefault())
							contentView.ScrollToBeginning()
						}
					} else {
						// Refresh content if the changed file is currently selected
						sel := filesView.GetCurrentItem()
						if sel >= 0 && sel < len(flieLists) {
							cur := flieLists[sel]
							if changedBase == cur {
								contentView.SetText(readFile(filepath.Join(currentDir, cur)))
								contentView.ScrollToBeginning()
							}
						}
					}
				})
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("watcher.Errors", slog.String("error", err.Error()))
		}
	}
}
