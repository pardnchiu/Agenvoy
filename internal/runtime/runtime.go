package runtime

import (
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	stopGraceWindow = 3 * time.Second
	stopPollGap     = 100 * time.Millisecond
)

var ErrAlreadyRunning = errors.New("agenvoy daemon already running")

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
	if err := go_pkg_filesystem.WriteJSON(path(), r, false); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteJSON", "", err)
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
			return existing, ErrAlreadyRunning
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

func Stop(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os.FindProcess", "", err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "proc.SIGTERM", "", err)
	}
	deadline := time.Now().Add(stopGraceWindow)
	for time.Now().Before(deadline) {
		if !IsAlive(pid) {
			return nil
		}
		time.Sleep(stopPollGap)
	}
	if err := proc.Signal(syscall.SIGKILL); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "proc.SIGKILL", "", err)
	}
	return nil
}

func ListDaemons() ([]int, error) {
	out, err := osexec.Command("ps", "-axo", "pid=,args=").Output()
	if err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Unavailable, "ps", "", err)
	}
	self := os.Getpid()
	var pids []int
	for line := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != "--daemon" {
			continue
		}
		name := filepath.Base(fields[1])
		if name != "agen" && !(name == "app" && strings.Contains(fields[1], "go-build")) {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid == self {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func Clear() error {
	p := path()
	if !go_pkg_filesystem_reader.Exists(p) {
		return nil
	}
	if err := os.Remove(p); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os.Remove", "", err)
	}
	return nil
}

func IsCurrent() bool {
	r, err := Read()
	if err != nil || r == nil {
		return false
	}
	return IsAlive(r.PID)
}
