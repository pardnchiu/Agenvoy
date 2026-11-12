package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	osexec "os/exec"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/mcp"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "stop":
			stop()
			return

		case "update":
			update()
			return

		case "--daemon":
			Daemon()
			return

		default:
			if strings.HasPrefix(os.Args[1], "-") {
				prompt(os.Args[1:])
				return
			}
			usage()
			os.Exit(1)
		}
	}

	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
		mcpServer()
		return
	}

	TUI()
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  agen                                            Attach TUI; spawn server daemon if not running")
	fmt.Println("  agen stop                                       Stop the running server daemon")
	fmt.Println("  agen -p <prompt> [flags]                        Send one prompt to the daemon and print the reply")
	fmt.Println("      -m, --model <name>                          Model (default: auto)")
	fmt.Println("      -r, --reasoning <level>                     Reasoning level (default: medium)")
	fmt.Println("      --stream                                    Print the reply as it streams")
	fmt.Println("      --work-dir <dir>                            Working directory (default: current directory)")
	fmt.Println("      --allow-edit                                Allow writes and non-read-only commands")
	fmt.Println("      --skip-sandbox                              Allow paths outside $HOME, sudo and sandbox: false")
	fmt.Println("  agen update                                     Update agen to the latest release")
}

func stop() {
	if err := filesystem.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "filesystem.Init: %v\n", err)
		os.Exit(1)
	}
	pids, err := runtime.ListDaemons()
	if err != nil {
		fmt.Fprintf(os.Stderr, "runtime.ListDaemons: %v\n", err)
	}
	if r, err := runtime.Read(); err == nil && r != nil && runtime.IsAlive(r.PID) && !slices.Contains(pids, r.PID) {
		pids = append(pids, r.PID)
	}
	if len(pids) == 0 {
		fmt.Println("No daemon running.")
		_ = runtime.Clear()
		return
	}
	failed := false
	for _, pid := range pids {
		fmt.Printf("Stopping daemon (pid=%d)...\n", pid)
		if err := runtime.Stop(pid); err != nil {
			fmt.Fprintf(os.Stderr, "runtime.Stop: %v\n", err)
			failed = true
		}
	}
	if err := runtime.Clear(); err != nil {
		slog.Warn("runtime.Clear",
			slog.String("error", err.Error()))
	}
	if failed {
		os.Exit(1)
	}
	fmt.Printf("Stopped %d daemon(s).\n", len(pids))
}

func update() {
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

	fmt.Printf("Fetching updater from %s -> %s\n", configs.ENDPOINT_UPDATE_SHELL, tmpPath)
	curl := osexec.Command("curl", "-fsSL", configs.ENDPOINT_UPDATE_SHELL, "-o", tmpPath)
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

func mcpServer() {
	if err := filesystem.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "filesystem.Init: %v\n", err)
		os.Exit(1)
	}
	if err := filesystem.LoadRuntime(); err != nil {
		slog.Warn("filesystem.LoadRuntime",
			slog.String("error", err.Error()))
	}
	if err := go_pkg_sandbox.CheckDependence(); err != nil {
		slog.Warn("sandbox.CheckDependence",
			slog.String("error", err.Error()))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	server := mcp.NewServer()
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "mcpserver: %v\n", err)
		os.Exit(1)
	}
}
