package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	osexec "os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	geminiStt "github.com/pardnchiu/agenvoy/internal/agents/provider/gemini/stt"
	geminiYoutube "github.com/pardnchiu/agenvoy/internal/agents/provider/gemini/youtube"
	codexImage2 "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex/image2"
	"github.com/pardnchiu/agenvoy/internal/agents/summary"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/cli"
	"github.com/pardnchiu/agenvoy/internal/runtime/discord"
	"github.com/pardnchiu/agenvoy/internal/runtime/telegram"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/toolAdapter/mcp"
	"github.com/pardnchiu/agenvoy/internal/tools/agent/subagent"
)

func init() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("godotenv.Load",
			slog.String("error", err.Error()))
	}
	go_pkg_sandbox.New(configs.DeniedMap)
	if err := go_pkg_filesystem.New(go_pkg_filesystem.Policy{
		DeniedMap:   configs.DeniedMap,
		ExcludeList: configs.ExcludeList,
	}); err != nil {
		slog.Error("go_pkg_filesystem.New",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	exec.RegisterPushHook("dc-", discord.PushDiscordResult)
	exec.RegisterPushHook("tg-", telegram.PushTelegramResult)
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "model":
			initCLI()
			runModel(os.Args[2:])
			return

		case "session":
			initCLI()
			cli.Session(os.Args[2:])
			return

		case "mcp":
			initCLI()
			cli.MCP(os.Args[2:])
			return

		case "cli", "run":
			if len(os.Args) < 3 {
				fmt.Fprintf(os.Stderr, "Usage: agen cli <input...>\n")
				fmt.Fprintf(os.Stderr, "       agen run <input...>\n")
				os.Exit(1)
			}
			initCLI()
			cmdAgent(os.Args[1] == "run")
			return

		case "stop":
			runStop()
			return

		case "update":
			runUpdate()
			return

		case "--daemon":
			cmdDaemon()
			return

		default:
			printUsage()
			os.Exit(1)
		}
	}

	newTUI()
}

func initCLI() {
	if err := go_pkg_sandbox.CheckDependence(); err != nil {
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
	codexImage2.Register()
	geminiYoutube.Register()
	geminiStt.Register()
}

func modelCheck() {
	cfg, err := session.Load()
	if err != nil {
		slog.Error("session.Load",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	if len(cfg.Models) == 0 {
		fmt.Println("[*] No model configured. Setting up first model…")
		runAdd()

		cfg, err = session.Load()
		if err != nil || len(cfg.Models) == 0 {
			fmt.Println("[!] No model added. Exiting.")
			os.Exit(0)
		}
	}

	checkModels()

	cfg, err = session.Load()
	if err != nil || len(cfg.Models) == 0 {
		fmt.Println("[!] No model remaining after cleanup. Exiting.")
		os.Exit(0)
	}
}

func runModel(args []string) {
	sub := ""
	if len(args) > 0 {
		sub = strings.ToLower(strings.TrimSpace(args[0]))
	}
	if sub == "" {
		sub = cli.Pick("Select model action", []string{"add", "remove", "list", "planner", "reasoning"})
	}
	switch sub {
	case "add":
		runAdd()
	case "remove", "rm":
		cli.RunRemove()
	case "list":
		runModelList()
	case "planner":
		runPlanner()
	case "reasoning":
		runReasoning()
	default:
		fmt.Fprintf(os.Stderr, "Usage: agen model [add|remove|list|planner|reasoning]\n")
		os.Exit(1)
	}
}

func runModelList() {
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

func runStop() {
	if err := filesystem.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "filesystem.Init: %v\n", err)
		os.Exit(1)
	}
	r, err := runtime.Read()
	if err != nil || r == nil {
		fmt.Println("No daemon running.")
		return
	}
	if !runtime.IsAlive(r.PID) {
		fmt.Printf("Daemon record stale (pid=%d not alive); clearing.\n", r.PID)
		_ = runtime.Clear()
		return
	}
	fmt.Printf("Stopping daemon (pid=%d)...\n", r.PID)
	if err := runtime.Stop(r.PID); err != nil {
		fmt.Fprintf(os.Stderr, "runtime.Stop: %v\n", err)
		os.Exit(1)
	}
	if err := runtime.Clear(); err != nil {
		slog.Warn("runtime.Clear",
			slog.String("error", err.Error()))
	}
	fmt.Println("Daemon stopped.")
}

func setSummaryCron() {
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
			summaryAgent := exec.SelectAgent(bgCtx, host.Planner(), host.Registry(), "[summary] background summary cron", false, sid)
			summary.Generate(bgCtx, summaryAgent, sid, summaryHistories)
			slog.Info("summary done",
				slog.String("session", sid))
		}
	}
}

func initMCP(ctx context.Context) *mcp.MCP {
	sessionID := ""
	if cfg, err := session.Load(); err == nil {
		sessionID = strings.TrimSpace(cfg.SessionID)
	}
	manager, err := mcp.New(ctx, sessionID)
	if err != nil {
		slog.Warn("mcp.New",
			slog.String("error", err.Error()))
		return nil
	}
	manager.RegisterAll(ctx)
	return manager
}

func clearSession() {
	idx, err := go_pkg_filesystem.ReadJSON[struct {
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
	fmt.Println("  agen                                            Attach TUI; spawn server daemon if not running")
	fmt.Println("  agen stop                                       Stop the running server daemon")
	fmt.Println("  agen update                                     Update agen to the latest release")
	fmt.Println("  agen model [add|remove|list|planner|reasoning]  Manage providers/models, planner, reasoning")
	fmt.Println("  agen mcp [list|add|remove]                      Manage MCP servers")
	fmt.Println("  agen session [new|switch|config] [name]         Manage CLI sessions (interactive picker if no name)")
	fmt.Println("  agen cli <input...>                             Run agent (requires tool confirmation)")
	fmt.Println("  agen run <input...>                             Run agent (allow all tools)")
}

func runUpdate() {
	const remoteURL = "https://cloud.agenvoy.com/update.sh"

	f, err := os.CreateTemp("", "agenvoy-update-*.sh")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp: %v\n", err)
		os.Exit(1)
	}
	tmpPath := f.Name()
	f.Close()

	cleanup := func() { _ = os.Remove(tmpPath) }
	defer cleanup()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cleanup()
		os.Exit(130)
	}()

	fmt.Printf("Fetching updater from %s -> %s\n", remoteURL, tmpPath)
	curl := osexec.Command("curl", "-fsSL", remoteURL, "-o", tmpPath)
	curl.Stdout = os.Stdout
	curl.Stderr = os.Stderr
	if err := curl.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "download failed: %v\n", err)
		os.Exit(1)
	}

	cmd := osexec.Command("bash", tmpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
		os.Exit(1)
	}
}
