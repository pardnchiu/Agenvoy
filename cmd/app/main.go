package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/pardnchiu/agenvoy/extensions"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/claude"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/compat"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/copilot"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/gemini"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/nvidia"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/openai"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/discord"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/routes"
	"github.com/pardnchiu/agenvoy/internal/sandbox"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
	"github.com/pardnchiu/agenvoy/internal/tui"
)

func init() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("godotenv.Load",
			slog.String("error", err.Error()))
	}
}

func main() {
	if err := filesystem.Init(); err != nil {
		slog.Error("filesystem.Init",
			slog.String("error", err.Error()))
		return
	}

	tui.New()
	tui.SetSlog()

	if err := sandbox.CheckDependence(); err != nil {
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

	registry := buildAgentRegistry()
	go skill.SyncSkills(context.Background(), extensions.Skills)
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

	if selectorBot != nil {
		slog.Info("agent registry built",
			slog.Int("entries", len(registry.Entries)),
			slog.String("fallback", selectorBot.Name()))

		bot, err := discord.New(selectorBot, registry, scanner)
		if err != nil {
			slog.Error("discord.New",
				slog.String("error", err.Error()))
		} else if bot == nil {
			slog.Warn("DISCORD_TOKEN not set, bot disabled")
		}

		route := routes.New(selectorBot, registry, scanner)

		port := os.Getenv("PORT")
		if port == "" {
			port = "17989"
		}

		server := &http.Server{
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

		defer func() {
			scheduler.Stop()
			if bot != nil {
				discord.Close(bot)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()
	} else {
		slog.Warn("no agents configured, server and discord disabled")
		defer scheduler.Stop()
	}

	go tui.FileMonitor()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		tui.Stop()
	}()

	if err := tui.Set(); err != nil {
		fmt.Fprintf(os.Stderr, "tui.Set error: %v\n", err)
	}
}

func buildAgentRegistry() agentTypes.AgentRegistry {
	newFn := map[string]func(string) (agentTypes.Agent, error){
		"copilot": func(m string) (agentTypes.Agent, error) { return copilot.New(m) },
		"openai":  func(m string) (agentTypes.Agent, error) { return openai.New(m) },
		"compat":  func(m string) (agentTypes.Agent, error) { return compat.New(m) },
		"claude":  func(m string) (agentTypes.Agent, error) { return claude.New(m) },
		"gemini":  func(m string) (agentTypes.Agent, error) { return gemini.New(m) },
		"nvidia":  func(m string) (agentTypes.Agent, error) { return nvidia.New(m) },
	}

	agentEntries := exec.GetAgent()
	registry := agentTypes.AgentRegistry{
		Registry: make(map[string]agentTypes.Agent, len(agentEntries)),
		Entries:  make([]agentTypes.AgentEntry, 0, len(agentEntries)),
	}
	for _, e := range agentEntries {
		providerFull := strings.SplitN(e.Name, "@", 2)[0]
		prov, _, _ := strings.Cut(providerFull, "[")
		fn, ok := newFn[prov]
		if !ok {
			continue
		}
		a, err := fn(e.Name)
		if err != nil {
			slog.Warn("failed to initialize",
				slog.String("name", e.Name),
				slog.String("error", err.Error()))
			continue
		}
		registry.Registry[e.Name] = a
		registry.Entries = append(registry.Entries, e)
		if registry.Fallback == nil {
			registry.Fallback = a
		}
	}

	if registry.Fallback == nil {
		slog.Error("no agents initialized, please check API keys")
	}

	return registry
}
