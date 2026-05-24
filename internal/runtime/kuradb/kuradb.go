package kuradb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const (
	BinaryPath        = "/usr/local/bin/kura"
	healthInterval    = 1 * time.Minute
	healthRequestTime = 5 * time.Second
	healthMaxStrikes  = 3
)

func Remove() error {
	if err := os.Remove(filesystem.KuradbEndpointPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("os.Remove %s: %w", filesystem.KuradbEndpointPath, err)
	}
	return nil
}

func IsInstalled() bool {
	info, err := os.Stat(BinaryPath)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode()&0o111 != 0
}

var healthClient = &http.Client{
	Timeout: healthRequestTime,
}

func Health(ctx context.Context, onFail func()) {
	ticker := time.NewTicker(healthInterval)
	defer ticker.Stop()

	strikes := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := probeHealth(ctx); err != nil {
				strikes++
				slog.Warn("kuradb.RunHealth: probe failed",
					slog.Int("strike", strikes),
					slog.Int("max", healthMaxStrikes),
					slog.String("error", err.Error()))
				if strikes >= healthMaxStrikes {
					slog.Error("kuradb.RunHealth: 3 consecutive failures, disabling")
					if onFail != nil {
						onFail()
					}
					return
				}
				continue
			}
			if strikes > 0 {
				slog.Info("kuradb.RunHealth: recovered",
					slog.Int("prior_strikes", strikes))
			}
			strikes = 0
		}
	}
}

func probeHealth(ctx context.Context) error {
	base, err := filesystem.GetKuradbEndpoint()
	if err != nil {
		return fmt.Errorf("filesystem.GetKuradbEndpoint: %w", err)
	}
	reqCtx, cancel := context.WithTimeout(ctx, healthRequestTime)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, base+"/api/health", nil)
	if err != nil {
		return fmt.Errorf("http.NewRequestWithContext: %w", err)
	}
	resp, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthClient.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
