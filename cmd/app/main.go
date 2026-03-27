package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tui"
)

func main() {
	if err := filesystem.Init(); err != nil {
		slog.Error("filesystem.Init",
			slog.String("error", err.Error()))
		return
	}

	tui.New()
	tui.SetSlog()

	go tui.FileMonitor()
	go func() {
		if err := tui.Set(); err != nil {
			slog.Error("tui.Set", slog.String("error", err.Error()))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	tui.Stop()
}
