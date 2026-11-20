package sudo

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	cachedTimeout = 3 * time.Second
	verifyTimeout = 30 * time.Second
)

func Cached(ctx context.Context) bool {
	if os.Geteuid() == 0 {
		return true
	}
	probeCtx, cancel := context.WithTimeout(ctx, cachedTimeout)
	defer cancel()
	return exec.CommandContext(probeCtx, "sudo", "-n", "-v").Run() == nil
}

func Verify(ctx context.Context, password string) error {
	if os.Geteuid() == 0 {
		return nil
	}
	if Cached(ctx) {
		return nil
	}
	if password == "" {
		return utilsmsg.New(utilsmsg.Invalid, "password", "is required")
	}

	verifyCtx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	cmd := exec.CommandContext(verifyCtx, "sudo", "-S", "-p", "", "-v")
	cmd.Stdin = strings.NewReader(password + "\n")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if verifyCtx.Err() != nil {
		return utilsmsg.New(utilsmsg.Timeout, "password check", "timed out")
	}
	if strings.Contains(strings.ToLower(string(out)), "incorrect password") {
		return utilsmsg.New(utilsmsg.Denied, "password", "incorrect password")
	}
	return utilsmsg.New(utilsmsg.Internal, "password check", "failed")
}
