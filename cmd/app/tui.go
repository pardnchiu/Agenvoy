package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/app"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/tui"
	sessiontui "github.com/pardnchiu/agenvoy/internal/session/tui"
)

func TUI() {
	// lipgloss.SetHasDarkBackground(true)

	sessiontui.New()

	if err := filesystem.Init(); err != nil {
		slog.Error("filesystem.Init",
			slog.String("error", err.Error()))
		return
	}
	if err := filesystem.LoadRuntime(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\nReset %s to the initial config? The current file is kept as config.json.corrupt [y/N]: ", err, filesystem.ConfigPath)
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if !strings.EqualFold(strings.TrimSpace(answer), "y") {
			fmt.Fprintf(os.Stderr, "fix %s, then start again\n", filesystem.ConfigPath)
			os.Exit(1)
		}
		if err := filesystem.ResetConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}
	closeLocal, err := openLocal()
	if err != nil {
		slog.Error("openLocal",
			slog.String("error", err.Error()))
		return
	}
	defer closeLocal()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopWatcher := app.WatchConfig(ctx, func() {
		agents.Reload()
	})
	defer stopWatcher()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	go func() {
		<-quit
		cancel()
	}()

	if err := tui.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "tui.Run error: %v\n", err)
	}
}
