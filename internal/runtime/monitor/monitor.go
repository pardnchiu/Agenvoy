package monitor

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"runtime/metrics"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	tickInterval    = 30 * time.Second
	cpuWarnPercent  = 80.0
	ramWarnBytes    = uint64(2) << 30
	netProbeTarget  = "1.1.1.1:443"
	netProbeTimeout = 5 * time.Second
	topProcessCount = 3
	psTimeout       = 3 * time.Second
)

func Start(ctx context.Context) {
	go run(ctx)
}

func run(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	lastTotal, lastIdle, hasCPU := sampleCPU()

	var cpuOver, ramOver, netDown bool

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if curTotal, curIdle, ok := sampleCPU(); ok {
				if hasCPU && curTotal > lastTotal {
					dTotal := curTotal - lastTotal
					dIdle := curIdle - lastIdle
					pct := min(max((dTotal-dIdle)/dTotal*100, 0), 100)
					if pct >= cpuWarnPercent {
						attrs := []any{
							slog.Float64("percent", pct),
							slog.Int("cpus", goruntime.NumCPU()),
						}
						if top := topCPU(ctx, topProcessCount); top != "" {
							attrs = append(attrs, slog.String("top", top))
						}
						slog.Warn("monitor: high CPU", attrs...)
						cpuOver = true
					} else if cpuOver {
						slog.Info("monitor: CPU recovered",
							slog.Float64("percent", pct))
						cpuOver = false
					}
				}
				lastTotal, lastIdle = curTotal, curIdle
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

func sampleCPU() (total, idle float64, ok bool) {
	samples := []metrics.Sample{
		{Name: "/cpu/classes/total:cpu-seconds"},
		{Name: "/cpu/classes/idle:cpu-seconds"},
	}
	metrics.Read(samples)
	if samples[0].Value.Kind() != metrics.KindFloat64 || samples[1].Value.Kind() != metrics.KindFloat64 {
		return 0, 0, false
	}
	return samples[0].Value.Float64(), samples[1].Value.Float64(), true
}

func topCPU(ctx context.Context, n int) string {
	cmdCtx, cancel := context.WithTimeout(ctx, psTimeout)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "ps", "-Ao", "pid,pcpu,comm").Output()
	if err != nil {
		return ""
	}

	type proc struct {
		pid  string
		pcpu float64
		name string
	}
	var procs []proc
	first := true
	for line := range strings.SplitSeq(string(out), "\n") {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pcpu, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		procs = append(procs, proc{
			pid:  fields[0],
			pcpu: pcpu,
			name: filepath.Base(strings.Join(fields[2:], " ")),
		})
	}

	slices.SortFunc(procs, func(a, b proc) int { return cmp.Compare(b.pcpu, a.pcpu) })
	procs = procs[:min(n, len(procs))]
	parts := make([]string, 0, len(procs))
	for _, p := range procs {
		parts = append(parts, fmt.Sprintf("%s[%s] %.1f%%", p.name, p.pid, p.pcpu))
	}
	return strings.Join(parts, ", ")
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
