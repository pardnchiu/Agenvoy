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
	bwrapOnce    sync.Once
	isAvailable  bool
	unshareFlags []string
)

func checkBwrap() {
	if exec.Command("bwrap", "--ro-bind", "/", "/", "--", "/bin/true").Run() != nil {
		return
	}
	isAvailable = true

	candidates := []string{
		"--unshare-user",
		"--unshare-pid",
		"--unshare-ipc",
		"--unshare-uts",
		"--unshare-cgroup",
	}
	for _, flag := range candidates {
		cmd := exec.Command("bwrap", "--ro-bind", "/", "/", flag, "--", "/bin/true")
		if cmd.Run() == nil {
			unshareFlags = append(unshareFlags, flag)
		} else {
			slog.Warn("bwrap unavailable",
				slog.String("flag", flag))
		}
	}
}

func Wrap(binary string, args []string, workDir string) (string, []string, error) {
	bwrapOnce.Do(func() {
		checkBwrap()
		if !isAvailable {
			slog.Warn("bwrap unavailable")
		}
	})

	if !isAvailable {
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
		"--new-session",
		"--die-with-parent",
	}
	bwrapArgs = append(bwrapArgs, unshareFlags...)

	deniedDirs, deniedFiles := deniedPaths(homeDir)
	for _, d := range deniedDirs {
		bwrapArgs = append(bwrapArgs, "--tmpfs", d)
	}
	for _, f := range deniedFiles {
		bwrapArgs = append(bwrapArgs, "--ro-bind", "/dev/null", f)
	}

	bwrapArgs = append(bwrapArgs, "--", binary)

	return "bwrap", append(bwrapArgs, args...), nil
}
