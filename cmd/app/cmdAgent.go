package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/cli"
	"github.com/pardnchiu/agenvoy/internal/session"
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
	scanner := runtime.NewSkillScanner()
	defer cancel()

	var selectorBot agentTypes.Agent
	if cfg, err := session.Load(); err == nil && cfg.PlannerModel != "" {
		selectorBot = cli.SelectAgent(cfg.PlannerModel)
	}
	if selectorBot == nil {
		selectorBot = registry.Fallback
	}

	agents.Set(selectorBot, registry, scanner)

	go cli.NewPending(ctx)

	if err := cli.Run(func(ch chan<- agentTypes.Event) error {
		return exec.Run(ctx, selectorBot, registry, scanner, userInput, nil, nil, ch, allowAll, "", "", false)
	}); err != nil && ctx.Err() == nil {
		slog.Error("failed to execute",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
}
