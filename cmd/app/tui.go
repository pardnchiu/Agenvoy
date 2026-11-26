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

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"

	"github.com/pardnchiu/agenvoy/internal/agents"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/app"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/runtime/tui"
	sessiontui "github.com/pardnchiu/agenvoy/internal/session/tui"
	usagelog "github.com/pardnchiu/agenvoy/internal/session/usage"
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
	if err := filesystem.OpenDB(); err != nil {
		slog.Error("filesystem.OpenDB",
			slog.String("error", err.Error()))
		return
	}
	defer filesystem.CloseDB()

	if err := historystore.New(); err != nil {
		slog.Warn("historystore.New",
			slog.String("error", err.Error()))
	}
	defer historystore.Close()

	if err := usagelog.New(); err != nil {
		slog.Warn("usagelog.New",
			slog.String("error", err.Error()))
	}
	defer usagelog.Close()

	if !runtime.IsCurrent() {
		if err := app.SpawnDaemon(); err != nil {
			slog.Warn("daemon launch failed; running TUI without server",
				slog.String("error", err.Error()))
		}
	}

	if err := torii.Init(filesystem.StoreDir); err != nil {
		slog.Error("store.Init",
			slog.String("error", err.Error()))
		return
	}
	defer torii.Close()

	if err := go_pkg_sandbox.CheckDependence(); err != nil {
		slog.Error("sandbox.CheckDependence",
			slog.String("error", err.Error()))
	}

	registry := app.NewAgentRegistry()
	scanner := runtime.NewSkillScanner()

	agents.Set(nil, app.SelectSummary(registry), registry, scanner)
	agents.SetRefresher(func() (agenttypes.Agent, agenttypes.Agent, agenttypes.ModelSet) {
		registry := app.NewAgentRegistry()
		return nil, app.SelectSummary(registry), registry
	})
	agents.MarkLoaded()

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
