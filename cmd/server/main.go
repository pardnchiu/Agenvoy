package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

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
	openaicodex "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/discord"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/sandbox"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
)

func init() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("godotenv.Load",
			slog.String("error", err.Error()))
	}
}

func main() {
	if err := sandbox.CheckDependence(); err != nil {
		slog.Error("sandbox.CheckDependence",
			slog.String("error", err.Error()))
		return
	}

	if err := filesystem.Init(); err != nil {
		slog.Error("filesystem.Init",
			slog.String("error", err.Error()))
		return
	}

	if err := scheduler.New(); err != nil {
		slog.Error("scheduler.New",
			slog.String("error", err.Error()))
		return
	}

	if err := tasks.Setup(scheduler.Get()); err != nil {
		slog.Warn("tasks.Setup",
			slog.String("error", err.Error()))
	}

	if err := crons.Setup(scheduler.Get()); err != nil {
		slog.Warn("crons.Setup",
			slog.String("error", err.Error()))
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

	slog.Info("agent registry built",
		slog.Int("entries", len(registry.Entries)),
		slog.String("fallback", registry.Fallback.Name()))
	bot, err := discord.New(selectorBot, registry, scanner)
	if bot != nil {
		defer discord.Close(bot)
	}
	if err != nil {
		slog.Error("failed to start bot", slog.String("error", err.Error()))
		return
	}
	if bot == nil {
		slog.Warn("DISCORD_TOKEN not set, bot disabled")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("signal received, shutting down")
	scheduler.Stop()
}

func buildAgentRegistry() agentTypes.AgentRegistry {
	newFn := map[string]func(string) (agentTypes.Agent, error){
		"copilot": func(m string) (agentTypes.Agent, error) { return copilot.New(m) },
		"openai":  func(m string) (agentTypes.Agent, error) { return openai.New(m) },
		"codex":   func(m string) (agentTypes.Agent, error) { return openaicodex.New(m) },
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
		provider, _, _ := strings.Cut(providerFull, "[")
		fn, ok := newFn[provider]
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
		slog.Error("please check API keys")
		os.Exit(1)
	}

	return registry
}
