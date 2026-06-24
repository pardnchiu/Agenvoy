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
	codexImage2 "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex/image2"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	chatbotTool "github.com/pardnchiu/agenvoy/internal/runtime/chatbot/tool"
	kuradbTool "github.com/pardnchiu/agenvoy/internal/runtime/kuradb/tool"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/runtime/tui"
	"github.com/pardnchiu/agenvoy/internal/toolAdapter/mcp"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	historyStore "github.com/pardnchiu/agenvoy/internal/session/history/store"
	tuiHash "github.com/pardnchiu/agenvoy/internal/session/tui"
	"github.com/pardnchiu/agenvoy/internal/tools/agent/plan"
	"github.com/pardnchiu/agenvoy/internal/tools/agent/subagent"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
)

func newTUI(initialInput string, onceCall, allowAll bool) {
	lipgloss.SetHasDarkBackground(true)

	tuiHash.New()

	if err := filesystem.Init(); err != nil {
		slog.Error("filesystem.Init",
			slog.String("error", err.Error()))
		return
	}
	if err := filesystem.LoadRuntime(); err != nil {
		slog.Warn("filesystem.LoadRuntime",
			slog.String("error", err.Error()))
	}
	if err := config.BackfillKeys(); err != nil {
		slog.Warn("session.BackfillKeys",
			slog.String("error", err.Error()))
	}
	if err := torii.Init(filesystem.StoreDir); err != nil {
		slog.Error("store.Init",
			slog.String("error", err.Error()))
		return
	}
	defer torii.Close()

	if err := historyStore.New(filesystem.HistoryDBPath); err != nil {
		slog.Warn("historyStore.Init",
			slog.String("error", err.Error()))
	}
	defer historyStore.Close()

	codexImage2.Register()
	geminiStt.Register()
	chatbotTool.Register()
	kuradbTool.Register()

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

	if cfg, err := config.Load(); err == nil {
		provider.SetReasoningLevel(cfg.ReasoningLevel)
	}
	subagent.Register()
	plan.Register()

	mcpManager := initMCP(context.Background(), "")
	defer mcpManager.Close()
	mcp.SetManager(mcpManager)

	registry := buildAgentRegistry()
	scanner := runtime.NewSkillScanner()
	selectorBot := dispatcherSelector(registry)
	summaryBot := summarySelector(registry)

	agents.Set(selectorBot, summaryBot, registry, scanner)
	agents.SetRefresher(refreshHost)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		cancel()
	}()

	if err := tui.Run(ctx, initialInput, onceCall, allowAll); err != nil {
		fmt.Fprintf(os.Stderr, "tui.Run error: %v\n", err)
	}
}
