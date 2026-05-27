package kuradb

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"time"
)

const (
	restartBackoff = 5 * time.Second
)

func Run(ctx context.Context) {
	if !IsInstalled() {
		slog.Warn("KuraDB not installed")
		return
	}

	for {
		if err := ctx.Err(); err != nil {
			return
		}

		cmd := exec.CommandContext(ctx, BinaryPath)
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			slog.Warn("KuraDB start failed",
				slog.String("error", err.Error()))
			waitBackoff(ctx)
			continue
		}

		go streamLog(stdout, slog.LevelInfo, "kura.stdout")
		go streamLog(stderr, slog.LevelInfo, "kura.stderr")

		err := cmd.Wait()

		if ctx.Err() != nil {
			return
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			slog.Warn("KuraDB exited")
		} else if err != nil {
			slog.Warn("KuraDB crashed",
				slog.String("error", err.Error()))
		} else {
			slog.Warn("KuraDB restarting")
		}

		waitBackoff(ctx)
	}
}

func streamLog(r io.ReadCloser, level slog.Level, prefix string) {
	if r == nil {
		return
	}
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		slog.Log(context.Background(), level, prefix,
			slog.String("line", scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("KuraDB",
			slog.String("prefix", prefix),
			slog.String("error", err.Error()))
	}
}

func waitBackoff(ctx context.Context) {
	select {
	case <-time.After(restartBackoff):
	case <-ctx.Done():
	}
}
