package runtime

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

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
	r, err := go_pkg_filesystem.ReadJSON[Runtime](path())
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func write(r *Runtime) error {
	if err := go_pkg_filesystem.WriteJSON(path(), r, true); err != nil {
		return fmt.Errorf("go_pkg_filesystem.WriteJSON: %w", err)
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
		UID:       go_pkg_utils.UUID(),
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
