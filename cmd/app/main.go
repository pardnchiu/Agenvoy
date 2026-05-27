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

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/summary"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/discord"
	"github.com/pardnchiu/agenvoy/internal/runtime/telegram"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/toolAdapter/mcp"
)

func init() {
	exec.RegisterPushHook("dc-", discord.PushDiscordResult)
	exec.RegisterPushHook("tg-", telegram.PushTelegramResult)
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "cli", "run":
			if len(os.Args) < 3 {
				fmt.Fprintf(os.Stderr, "Usage: agen cli <input...>\n")
				fmt.Fprintf(os.Stderr, "       agen run <input...>\n")
				os.Exit(1)
			}
			input := strings.TrimSpace(strings.ReplaceAll(strings.Join(os.Args[2:], " "), `\n`, "\n"))
			newTUI(input, true, os.Args[1] == "run")
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

	newTUI("", false, false)
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

		for _, sid := range sessions {
			histories, _ := session.GetHistory(sid)
			summaryHistories := summary.Get(histories)
			if len(summaryHistories) == 0 {
				continue
			}
			bgCtx := context.Background()
			summaryAgent := exec.SelectAgent(bgCtx, agents.Dispatcher(), agents.Registry(), "[summary] background summary cron", false, sid)
			if summaryAgent == nil {
				continue
			}
			summary.Generate(bgCtx, summaryAgent, sid, summaryHistories)
		}
	}
}

func initMCP(ctx context.Context, sessionID string) *mcp.MCP {
	manager, err := mcp.New(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		slog.Warn("mcp.New",
			slog.String("error", err.Error()))
		return nil
	}
	manager.RegisterAll(ctx)
	return manager
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  agen                                            Attach TUI; spawn server daemon if not running")
	fmt.Println("  agen stop                                       Stop the running server daemon")
	fmt.Println("  agen update                                     Update agen to the latest release")
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
