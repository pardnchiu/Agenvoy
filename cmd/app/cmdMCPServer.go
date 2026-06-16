package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/mcpserver"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
)

func cmdMCPServer() {
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

	server := mcpserver.New()
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "mcpserver: %v\n", err)
		os.Exit(1)
	}
}
