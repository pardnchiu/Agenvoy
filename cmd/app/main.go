package main

import (
	"bufio"
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
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "stop":
			stop(slices.Contains(os.Args[2:], "-y"))
			return

		case "update":
			update(slices.Contains(os.Args[2:], "-y"))
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
	fmt.Println("  agen stop [-y]                                  Stop the running server daemon; -y skips the running-task confirmation")
	fmt.Println("  agen -p <prompt> [flags]                        Run one prompt in this process and print the reply")
	fmt.Println("      -m, --model <name>                          Model (default: auto)")
	fmt.Println("      -r, --reasoning <level>                     Reasoning level (default: medium)")
	fmt.Println("      --stream                                    Print the reply as it streams")
	fmt.Println("      --work-dir <dir>                            Working directory (default: current directory)")
	fmt.Println("      --allow-edit                                Allow writes and non-read-only commands")
	fmt.Println("      --skip-sandbox                              DANGEROUS: leave the sandbox (paths outside $HOME, sudo, sandbox: false) with no confirmation; terminal only, not over HTTP")
	fmt.Println("  agen update [-y]                                Update agen to the latest release; -y skips the running-task confirmation")
}

func stop(yes bool) {
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
	if !yes && isTerminal() && !confirmInterrupt("stop") {
		os.Exit(1)
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

func update(yes bool) {
	if !yes && !confirmInterrupt("update") {
		os.Exit(1)
	}

	f, err := os.CreateTemp("", "agenvoy-update-*.sh")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Internal] os: CreateTemp: %v\n", err)
		os.Exit(1)
	}
	tmpPath := f.Name()
	_, err = f.Write(configs.UpdateShell)
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

	if err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "[Internal] os: Write: %v\n", err)
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

func isTerminal() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		if fi, err := f.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
			return false
		}
	}
	return true
}

func confirmInterrupt(action string) bool {
	if err := filesystem.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "[Internal] filesystem: Init: %v\n", err)
		os.Exit(1)
	}
	if err := torii.Init(filesystem.StoreDir); err != nil {
		fmt.Fprintf(os.Stderr, "[Unavailable] torii: Init: %v\n", err)
		os.Exit(1)
	}
	tasks := interactive.ListActiveTasks()
	torii.Close()
	if len(tasks) == 0 {
		return true
	}

	fmt.Printf("%d task(s) will be interrupted:\n", len(tasks))
	for _, one := range tasks {
		fmt.Printf("  - %s\n", one)
	}
	fmt.Printf("Type y to interrupt them and %s: ", action)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.TrimSpace(strings.ToLower(answer)) != "y" {
		fmt.Printf("%s canceled\n", action)
		return false
	}
	return true
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
