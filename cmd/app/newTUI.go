package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/lipgloss"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	geminiStt "github.com/pardnchiu/agenvoy/internal/agents/provider/gemini/stt"
	geminiYoutube "github.com/pardnchiu/agenvoy/internal/agents/provider/gemini/youtube"
	codexImage2 "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex/image2"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	discordTool "github.com/pardnchiu/agenvoy/internal/runtime/discord/tool"
	telegramTool "github.com/pardnchiu/agenvoy/internal/runtime/telegram/tool"
	"github.com/pardnchiu/agenvoy/internal/runtime/tui"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/tools/agent/plan"
	"github.com/pardnchiu/agenvoy/internal/tools/agent/subagent"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
)

func newTUI() {
	lipgloss.SetHasDarkBackground(true)

	session.SetHash(session.Hash())

	if err := filesystem.Init(); err != nil {
		slog.Error("filesystem.Init",
			slog.String("error", err.Error()))
		return
	}
	if err := torii.Init(filesystem.StoreDir); err != nil {
		slog.Error("store.Init",
			slog.String("error", err.Error()))
		return
	}
	defer torii.Close()

	codexImage2.Register()
	geminiYoutube.Register()
	geminiStt.Register()
	telegramTool.Register()
	discordTool.Register()

	modelCheck()

	if !runtime.IsCurrent() {
		if err := newDaemon(); err != nil {
			slog.Warn("daemon launch failed; running TUI without server",
				slog.String("error", err.Error()))
		}
	}

	if err := go_pkg_sandbox.CheckDependence(); err != nil {
		slog.Error("sandbox.CheckDependence",
			slog.String("error", err.Error()))
	}

	if cfg, err := session.Load(); err == nil {
		provider.SetReasoningLevel(cfg.ReasoningLevel)
	}
	subagent.Register()
	plan.Register()

	mcpManager := initMCP(context.Background(), "")
	defer mcpManager.Close()

	registry := buildAgentRegistry()
	scanner := runtime.NewSkillScanner()
	selectorBot := dispatcherSelector(registry)

	agents.Set(selectorBot, registry, scanner)
	agents.SetRefresher(refreshHost)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		cancel()
	}()

	if err := tui.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "tui.Run error: %v\n", err)
	}
}
