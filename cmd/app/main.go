package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"
	go_utils_sandbox "github.com/pardnchiu/go-utils/sandbox"
	go_utils_utils "github.com/pardnchiu/go-utils/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/extensions"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/agenvoy/internal/agents/summary"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
	"github.com/pardnchiu/agenvoy/internal/interactive/cli"
	"github.com/pardnchiu/agenvoy/internal/interactive/discord"
	"github.com/pardnchiu/agenvoy/internal/routes"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
	"github.com/pardnchiu/agenvoy/internal/tools/agent/subagent"
	"github.com/pardnchiu/agenvoy/internal/tui"
)

func init() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("godotenv.Load",
			slog.String("error", err.Error()))
	}
	go_utils_sandbox.New(configs.DeniedMap)
	if err := go_utils_filesystem.New(go_utils_filesystem.Policy{
		DeniedMap:   configs.DeniedMap,
		ExcludeList: configs.ExcludeList,
	}); err != nil {
		slog.Error("go_utils_filesystem.New",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "add":
			initCLI()
			runAdd()
			return
		case "remove":
			initCLI()
			cli.RunRemove()
			return
		case "reasoning":
			initCLI()
			runReasoning()
			return
		case "planner":
			initCLI()
			runPlanner()
			return
		case "list":
			initCLI()
			runList()
			return
		case "config":
			initCLI()
			cli.Config()
			return
		case "new":
			initCLI()
			name := ""
			if len(os.Args) >= 3 {
				name = os.Args[2]
			}
			cli.NewSession(name)
			return
		case "switch":
			if len(os.Args) < 3 {
				fmt.Fprintf(os.Stderr, "Usage: agen switch <name>\n")
				os.Exit(1)
			}
			initCLI()
			cli.Switch(os.Args[2])
			return
		case "cli", "run":
			if len(os.Args) < 3 {
				fmt.Fprintf(os.Stderr, "Usage: agen cli <input...>\n")
				fmt.Fprintf(os.Stderr, "       agen run <input...>\n")
				os.Exit(1)
			}
			initCLI()
			runAgent(os.Args[1] == "run")
			return
		default:
			printUsage()
			os.Exit(1)
		}
	}

	runApp()
}

func initCLI() {
	if err := go_utils_sandbox.CheckDependence(); err != nil {
		slog.Error("sandbox.CheckDependence",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := filesystem.Init(); err != nil {
		slog.Error("filesystem.Init",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := torii.Init(filesystem.StoreDir); err != nil {
		slog.Error("store.Init",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	if cfg, err := session.Load(); err == nil {
		provider.SetReasoningLevel(cfg.ReasoningLevel)
	}
	subagent.Register()
}

func runList() {
	if len(os.Args) > 2 && os.Args[2] == "skill" {
		skill.SyncSkills(context.Background(), extensions.Skills)
		scanner := skill.NewScanner()

		if len(scanner.Skills.ByName) == 0 {
			fmt.Println("No skills found")
			fmt.Println("\nScanned paths:")
			for _, path := range scanner.Skills.Paths {
				fmt.Printf("  - %s\n", path)
			}
			return
		}

		names := scanner.List()
		sort.Strings(names)

		fmt.Printf("Found %d skill(s):\n\n", len(names))
		for _, name := range names {
			s := scanner.Skills.ByName[name]
			fmt.Printf("• %s\n", name)
			if s.Description != "" {
				fmt.Printf("  %s\n", s.Description)
			}
			fmt.Printf("  Path: %s\n\n", s.Path)
		}
		return
	}

	cfg, err := session.Load()
	if err != nil {
		slog.Error("session.Load", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if len(cfg.Models) == 0 {
		fmt.Println("No models configured.")
		return
	}

	fmt.Printf("Found %d model(s):\n\n", len(cfg.Models))
	for _, m := range cfg.Models {
		fmt.Printf("• %s\n", m.Name)
		if m.Description != "" {
			fmt.Printf("  %s\n", m.Description)
		}
	}
}

func runAgent(allowAll bool) {
	defer torii.Close()

	if !runtime.IsCurrent() {
		clearSession()
	}

	userInput := strings.TrimSpace(strings.ReplaceAll(strings.Join(os.Args[2:], " "), `\n`, "\n"))

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

	if err := cli.Run(ctx, cancel, func(ch chan<- agentTypes.Event) error {
		return exec.Run(ctx, selectorBot, registry, scanner, userInput, nil, nil, ch, allowAll)
	}); err != nil && ctx.Err() == nil {
		slog.Error("failed to execute",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func runApp() {
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
		slog.Warn("runtime.Init",
			slog.String("error", err.Error()))
	}
	session.Clean()
	session.CleanAllTask()

	tui.New()
	tui.SetSlog()

	if err := go_utils_sandbox.CheckDependence(); err != nil {
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

	registry := buildAgentRegistry()
	skill.SyncSkills(context.Background(), extensions.Skills)
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

		port := go_utils_utils.GetWithDefault("PORT", "17989")

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

	if selectorBot != nil {
		go setSummaryCron(selectorBot, registry)
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

func setSummaryCron(bot agentTypes.Agent, registry agentTypes.AgentRegistry) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sessions := session.IsNeedSummary()
		if len(sessions) == 0 {
			continue
		}
		slog.Info("summary cron",
			slog.Int("sessions", len(sessions)))

		for _, sid := range sessions {
			histories, _ := session.GetHistory(sid)
			summaryHistories := summary.Get(histories)
			if len(summaryHistories) == 0 {
				continue
			}
			bgCtx := context.Background()
			summaryAgent := exec.SelectAgent(bgCtx, bot, registry, "[summary] 整理對話摘要，選擇最輕量可完成任務的模型", false)
			summary.Generate(bgCtx, summaryAgent, sid, summaryHistories)
			slog.Info("summary done",
				slog.String("session", sid))
		}
	}
}

func clearSession() {
	idx, err := go_utils_filesystem.ReadJSON[struct {
		SessionID string `json:"session_id"`
	}](filesystem.ConfigPath)
	if err != nil {
		return
	}
	sid := strings.TrimSpace(idx.SessionID)
	if sid == "" {
		return
	}
	session.ClearTask(sid)
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  agen                    Start TUI + server + Discord bot")
	fmt.Println("  agen add                Add a provider/model")
	fmt.Println("  agen remove             Remove a provider/model")
	fmt.Println("  agen list               List configured models")
	fmt.Println("  agen list skill         List available skills")
	fmt.Println("  agen config             Edit current CLI session bot.md in $EDITOR")
	fmt.Println("  agen new [name]         Start a new CLI session (optional bot.md name) and switch primary pointer")
	fmt.Println("  agen switch <name>      Switch primary pointer to the cli- session whose bot.md name matches")
	fmt.Println("  agen planner            Set planner model")
	fmt.Println("  agen reasoning          Set reasoning level")
	fmt.Println("  agen cli <input...>     Run agent (requires tool confirmation)")
	fmt.Println("  agen run <input...>     Run agent (allow all tools)")
}
