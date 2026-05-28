package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	goruntime "runtime"
	"runtime/metrics"
	"time"
)

const (
	tickInterval    = 30 * time.Second
	cpuWarnPercent  = 80.0
	ramWarnBytes    = uint64(2) << 30
	netProbeTarget  = "1.1.1.1:443"
	netProbeTimeout = 5 * time.Second
)

func Start(ctx context.Context) {
	go run(ctx)
}

func run(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	lastCPU, hasCPU := sampleCPUSeconds()
	lastTick := time.Now()

	var cpuOver, ramOver, netDown bool

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if cur, ok := sampleCPUSeconds(); ok {
				if hasCPU {
					elapsed := now.Sub(lastTick).Seconds()
					if elapsed > 0 {
						pct := (cur - lastCPU) / elapsed / float64(goruntime.NumCPU()) * 100
						if pct >= cpuWarnPercent {
							slog.Warn("monitor: high CPU",
								slog.Float64("percent", pct),
								slog.Int("cpus", goruntime.NumCPU()))
							cpuOver = true
						} else if cpuOver {
							slog.Info("monitor: CPU recovered",
								slog.Float64("percent", pct))
							cpuOver = false
						}
					}
				}
				lastCPU = cur
				lastTick = now
				hasCPU = true
			}

			var m goruntime.MemStats
			goruntime.ReadMemStats(&m)
			if m.Sys >= ramWarnBytes {
				slog.Warn("monitor: high RAM",
					slog.String("sys", humanBytes(m.Sys)),
					slog.String("alloc", humanBytes(m.Alloc)))
				ramOver = true
			} else if ramOver {
				slog.Info("monitor: RAM recovered",
					slog.String("sys", humanBytes(m.Sys)))
				ramOver = false
			}

			if err := probeNetwork(ctx); err != nil {
				slog.Warn("monitor: no network",
					slog.String("target", netProbeTarget),
					slog.String("error", err.Error()))
				netDown = true
			} else if netDown {
				slog.Info("monitor: network recovered",
					slog.String("target", netProbeTarget))
				netDown = false
			}
		}
	}
}

func sampleCPUSeconds() (float64, bool) {
	samples := []metrics.Sample{{Name: "/cpu/classes/total:cpu-seconds"}}
	metrics.Read(samples)
	if samples[0].Value.Kind() != metrics.KindFloat64 {
		return 0, false
	}
	return samples[0].Value.Float64(), true
}

func probeNetwork(ctx context.Context) error {
	probeCtx, cancel := context.WithTimeout(ctx, netProbeTimeout)
	defer cancel()
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(probeCtx, "tcp", netProbeTarget)
	if err != nil {
		return err
	}
	return conn.Close()
}

func humanBytes(b uint64) string {
	const (
		k = 1024
		m = k * 1024
		g = m * 1024
	)
	switch {
	case b >= g:
		return fmt.Sprintf("%.2fGiB", float64(b)/float64(g))
	case b >= m:
		return fmt.Sprintf("%.2fMiB", float64(b)/float64(m))
	case b >= k:
		return fmt.Sprintf("%.2fKiB", float64(b)/float64(k))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
