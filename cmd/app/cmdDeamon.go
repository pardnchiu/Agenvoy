package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/host"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
	"github.com/pardnchiu/agenvoy/internal/interactive/discord"
	discordTypes "github.com/pardnchiu/agenvoy/internal/interactive/discord/types"
	"github.com/pardnchiu/agenvoy/internal/routes"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
	"github.com/pardnchiu/agenvoy/internal/tools/agent/subagent"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
)

func cmdDaemon() {
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

	if _, err := runtime.Init(); err != nil {
		if errors.Is(err, runtime.ErrAlreadyRunning) {
			slog.Error("daemon already running, aborting")
			return
		}
		slog.Warn("runtime.Init",
			slog.String("error", err.Error()))
	}
	session.Clean()
	session.CleanAllTask()

	if err := go_pkg_sandbox.CheckDependence(); err != nil {
		slog.Error("sandbox.CheckDependence",
			slog.String("error", err.Error()))
	}

	if err := scheduler.New(); err != nil {
		slog.Error("scheduler.New",
			slog.String("error", err.Error()))
	} else {
		if err := tasks.Setup(scheduler.Get()); err != nil {
			slog.Warn("tasks.Setup",
				slog.String("error", err.Error()))
		}
		if err := crons.Setup(scheduler.Get()); err != nil {
			slog.Warn("crons.Setup",
				slog.String("error", err.Error()))
		}
	}

	if cfg, err := session.Load(); err == nil {
		provider.SetReasoningLevel(cfg.ReasoningLevel)
	}
	subagent.Register()

	mcpManager := initMCP(context.Background())
	defer mcpManager.Close()

	registry := buildAgentRegistry()
	scanner := skill.NewScanner()

	var selectorBot agentTypes.Agent
	if cfg, err := session.Load(); err == nil && cfg.PlannerModel != "" {
		if a, ok := registry.Registry[cfg.PlannerModel]; ok {
			selectorBot = a
		}
	}
	if selectorBot == nil {
		selectorBot = registry.Fallback
	}

	host.Set(selectorBot, registry, scanner)

	var bot *discordTypes.DiscordBot
	var server *http.Server

	if selectorBot != nil {
		slog.Info("agent registry built",
			slog.Int("entries", len(registry.Entries)),
			slog.String("fallback", selectorBot.Name()))

		b, err := discord.New(selectorBot, registry, scanner)
		if err != nil {
			slog.Error("discord.New",
				slog.String("error", err.Error()))
		} else if b == nil {
			slog.Warn("DISCORD_TOKEN not set, bot disabled")
		}
		bot = b

		route := routes.New(selectorBot, registry, scanner)
		port := go_pkg_utils.GetWithDefault("PORT", "17989")
		server = &http.Server{
			Addr:    ":" + port,
			Handler: route,
		}

		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("server.ListenAndServe",
					slog.String("error", err.Error()))
			}
		}()
		slog.Info("server started",
			slog.String("port", port))

		go setSummaryCron(selectorBot, registry)
	} else {
		slog.Warn("no agents configured, server and discord disabled")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("daemon shutting down")

	scheduler.Stop()
	if bot != nil {
		_ = discord.Close(bot)
	}
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = server.Shutdown(ctx)
		cancel()
	}
	if err := runtime.Clear(); err != nil {
		slog.Warn("runtime.Clear",
			slog.String("error", err.Error()))
	}
}
