package app

import (
	"os"
	"syscall"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func SpawnDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os.Executable", "", err)
	}

	logFile, err := os.OpenFile(filesystem.DaemonLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os.OpenFile", filesystem.DaemonLogPath, err)
	}
	defer logFile.Close()

	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os.OpenFile", os.DevNull, err)
	}
	defer devNull.Close()

	args := []string{exe, "--daemon"}
	proc, err := os.StartProcess(exe, args, &os.ProcAttr{
		Files: []*os.File{devNull, logFile, logFile},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	})
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os.StartProcess", exe, err)
	}
	if err := proc.Release(); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "proc.Release", "", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if r, err := runtime.Read(); err == nil && r != nil && runtime.IsAlive(r.PID) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return utilsmsg.New(utilsmsg.Timeout, "daemon", "did not become ready within 10s; check %s", filesystem.DaemonLogPath)
}
