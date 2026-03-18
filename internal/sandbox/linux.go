//go:build linux

package sandbox

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
)

// * if is nil, then install bubblewrap first
func CheckDependence() error {
	if _, err := exec.LookPath("bwrap"); err == nil {
		return nil
	}

	fmt.Println("please install bwrap first")

	var cmd *exec.Cmd
	switch {
	case checkBinary("apt-get"):
		cmd = exec.Command("sudo", "apt-get", "install", "-y", "bubblewrap")
	case checkBinary("dnf"):
		cmd = exec.Command("sudo", "dnf", "install", "-y", "bubblewrap")
	case checkBinary("yum"):
		cmd = exec.Command("sudo", "yum", "install", "-y", "bubblewrap")
	case checkBinary("pacman"):
		cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", "bubblewrap")
	case checkBinary("apk"):
		cmd = exec.Command("sudo", "apk", "add", "bubblewrap")
	default:
		return fmt.Errorf("os not supported")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmd.Run: %w", err)
	}

	if _, err := exec.LookPath("bwrap"); err != nil {
		return fmt.Errorf("exec.LookPath")
	}

	return nil
}

func checkBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

var (
	bwrapOnce  sync.Once
	isAvaiable bool
)

func checkBwrap() bool {
	cmd := exec.Command("bwrap", "--ro-bind", "/", "/", "--", "/bin/true")
	return cmd.Run() == nil
}

func Wrap(binary string, args []string, workDir string) (string, []string, error) {
	bwrapOnce.Do(func() {
		isAvaiable = checkBwrap()
		if !isAvaiable {
			slog.Warn("bwrap unavailable")
		}
	})

	if !isAvaiable {
		return binary, args, nil
	}

	homeDir, err := vaildateDir(workDir)
	if err != nil {
		return "", nil, err
	}

	bwrapArgs := []string{
		"--ro-bind", "/", "/",
		"--bind", homeDir, homeDir,
		"--tmpfs", "/tmp",
		"--dev", "/dev",
		"--proc", "/proc",
		"--share-net",
		"--die-with-parent",
		"--", binary,
	}

	return "bwrap", append(bwrapArgs, args...), nil
}
