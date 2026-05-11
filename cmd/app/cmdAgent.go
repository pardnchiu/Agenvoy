package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/extensions"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
	"github.com/pardnchiu/agenvoy/internal/interactive/cli"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
)

func cmdAgent(allowAll bool) {
	session.SetHash(session.Hash())

	defer torii.Close()

	modelCheck()

	if !runtime.IsCurrent() {
		clearSession()
	}

	userInput := strings.TrimSpace(strings.ReplaceAll(strings.Join(os.Args[2:], " "), `\n`, "\n"))

	mcpManager := initMCP(context.Background())
	defer mcpManager.Close()

	registry := buildAgentRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	skill.SyncSkills(ctx, extensions.Skills)
	scanner := skill.NewScanner()
	defer cancel()

	var selectorBot agentTypes.Agent
	if cfg, err := session.Load(); err == nil && cfg.PlannerModel != "" {
		selectorBot = cli.SelectAgent(cfg.PlannerModel)
	}
	if selectorBot == nil {
		selectorBot = registry.Fallback
	}

	host.Set(selectorBot, registry, scanner)

	go cli.NewPending(ctx)

	if err := cli.Run(func(ch chan<- agentTypes.Event) error {
		return exec.Run(ctx, selectorBot, registry, scanner, userInput, nil, nil, ch, allowAll, "", "", false)
	}); err != nil && ctx.Err() == nil {
		slog.Error("failed to execute",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
}
