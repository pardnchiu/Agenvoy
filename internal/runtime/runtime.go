package runtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"

	go_utils_utils "github.com/pardnchiu/go-utils/utils"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const (
	stopGraceWindow = 5 * time.Second
	stopPollGap     = 100 * time.Millisecond
)

type Runtime struct {
	UID       string `json:"uid"`
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"`
}

func path() string {
	return filepath.Join(filesystem.AgenvoyDir, "runtime.uid")
}

func Read() (*Runtime, error) {
	data, err := os.ReadFile(path())
	if err != nil {
		return nil, err
	}
	var r Runtime
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return &r, nil
}

func write(r *Runtime) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	if err := os.WriteFile(path(), data, 0644); err != nil {
		return fmt.Errorf("os.WriteFile: %w", err)
	}
	return nil
}

func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if pid == os.Getpid() {
		return true
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func Init() (*Runtime, error) {
	if existing, err := Read(); err == nil && existing != nil {
		if existing.PID != os.Getpid() && IsAlive(existing.PID) {
			slog.Warn("previous agenvoy instance still alive, terminating",
				slog.Int("pid", existing.PID),
				slog.String("uid", existing.UID))
			stopProcess(existing.PID)
		}
	}
	r := &Runtime{
		UID:       go_utils_utils.UUID(),
		PID:       os.Getpid(),
		StartedAt: time.Now().Format(time.RFC3339),
	}
	if err := write(r); err != nil {
		return nil, err
	}
	return r, nil
}

func stopProcess(pid int) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return
	}
	deadline := time.Now().Add(stopGraceWindow)
	for time.Now().Before(deadline) {
		if !IsAlive(pid) {
			return
		}
		time.Sleep(stopPollGap)
	}
	_ = proc.Signal(syscall.SIGKILL)
}

func IsCurrent() bool {
	r, err := Read()
	if err != nil || r == nil {
		return false
	}
	return IsAlive(r.PID)
}
