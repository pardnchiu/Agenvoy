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

	"github.com/pardnchiu/agenvoy/extensions"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
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
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "add":
			initCLI()
			runAdd()
			return
		case "remove":
			initCLI()
			runRemove()
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
	if err := sandbox.CheckDependence(); err != nil {
		slog.Error("sandbox.CheckDependence",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := filesystem.Init(); err != nil {
		slog.Error("filesystem.Init",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	if cfg, err := session.Load(); err == nil {
		provider.SetReasoningLevel(cfg.ReasoningLevel)
	}
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
	userInput := strings.TrimSpace(strings.ReplaceAll(strings.Join(os.Args[2:], " "), `\n`, "\n"))

	registry := buildAgentRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	skill.SyncSkills(ctx, extensions.Skills)
	scanner := skill.NewScanner()
	defer cancel()

	var selectorBot agentTypes.Agent
	if cfg, err := session.Load(); err == nil && cfg.PlannerModel != "" {
		selectorBot = selectAgent(cfg.PlannerModel)
	}
	if selectorBot == nil {
		selectorBot = registry.Fallback
	}

	if err := runEvents(ctx, cancel, func(ch chan<- agentTypes.Event) error {
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

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  agen                    Start TUI + server + Discord bot")
	fmt.Println("  agen add                Add a provider/model")
	fmt.Println("  agen remove             Remove a provider/model")
	fmt.Println("  agen list               List configured models")
	fmt.Println("  agen list skill         List available skills")
	fmt.Println("  agen planner            Set planner model")
	fmt.Println("  agen reasoning          Set reasoning level")
	fmt.Println("  agen cli <input...>     Run agent (requires tool confirmation)")
	fmt.Println("  agen run <input...>     Run agent (allow all tools)")
}
