package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const terminalLaunchTimeout = 10 * time.Second

const updateTerminalScript = `"$1" update || { printf '\nagen update failed; press Enter to close'; read _; }
`

var errNoTerminal = errors.New("no terminal available; run `agen update` in a terminal")

var displayEnvKeys = []string{"DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"}

var linuxTerminals = []struct {
	bin  string
	args []string
}{
	{"xdg-terminal-exec", nil},
	{"x-terminal-emulator", []string{"-e"}},
	{"gnome-terminal", []string{"--"}},
	{"konsole", []string{"-e"}},
	{"xfce4-terminal", []string{"-x"}},
	{"kitty", nil},
	{"alacritty", []string{"-e"}},
	{"wezterm", []string{"start", "--"}},
	{"xterm", []string{"-e"}},
}

func GetSystemUpdate() gin.HandlerFunc {
	return func(c *gin.Context) {
		latest, err := runtime.LatestRelease(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"version": runtime.CurrentVersion, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"version":          runtime.CurrentVersion,
			"latest":           latest,
			"update_available": latest != runtime.CurrentVersion,
		})
	}
}

func SystemUpdate() gin.HandlerFunc {
	return func(c *gin.Context) {
		exe, err := os.Executable()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "os.Executable: " + err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), terminalLaunchTimeout)
		defer cancel()

		switch {
		case goruntime.GOOS == "darwin":
			err = openMacTerminal(ctx, exe)
		case goruntime.GOOS == "linux" && utils.IsWSL():
			err = openWSLTerminal(ctx, exe)
		case goruntime.GOOS == "linux":
			err = openLinuxTerminal(ctx, exe)
		default:
			err = errNoTerminal
		}

		if errors.Is(err, errNoTerminal) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": err.Error()})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"status": "opened"})
	}
}

func openMacTerminal(ctx context.Context, exe string) error {
	command := "'" + strings.ReplaceAll(exe, "'", `'\''`) + "' update"
	quoted := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(command)
	script := "tell application \"Terminal\"\nactivate\ndo script \"" + quoted + "\"\nend tell"

	out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	if err != nil {
		return utilsmsg.New(utilsmsg.Internal, "osascript", "%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func openWSLTerminal(ctx context.Context, exe string) error {
	bin, err := exec.LookPath("cmd.exe")
	if err != nil {
		return errNoTerminal
	}

	script, err := writeUpdateTerminalScript()
	if err != nil {
		return err
	}

	args := []string{"/c", "start", "", "wsl.exe"}
	if distro := os.Getenv("WSL_DISTRO_NAME"); distro != "" {
		args = append(args, "-d", distro)
	}
	args = append(args, "--exec", "sh", script, exe)

	if err := exec.CommandContext(ctx, bin, args...).Run(); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "cmd.exe start", "", err)
	}
	return nil
}

func writeUpdateTerminalScript() (string, error) {
	path := filepath.Join(filesystem.AgenvoyDir, "update-terminal.sh")
	if err := go_pkg_filesystem.WriteFile(path, updateTerminalScript, 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteFile", "", err)
	}
	return path, nil
}

func openLinuxTerminal(ctx context.Context, exe string) error {
	env := displayEnv(ctx)
	if envValue(env, "DISPLAY") == "" && envValue(env, "WAYLAND_DISPLAY") == "" {
		return utilsmsg.New(utilsmsg.Unavailable, "graphical session", "not found (DISPLAY and WAYLAND_DISPLAY unset): %w", errNoTerminal)
	}

	script, err := writeUpdateTerminalScript()
	if err != nil {
		return err
	}

	for _, terminal := range linuxTerminals {
		bin, err := exec.LookPath(terminal.bin)
		if err != nil {
			continue
		}

		argv := append([]string{bin}, terminal.args...)
		argv = append(argv, "sh", script, exe)
		if os.Getenv("INVOCATION_ID") != "" {
			if scope, err := exec.LookPath("systemd-run"); err == nil {
				argv = append([]string{scope, "--user", "--scope", "--quiet", "--"}, argv...)
			}
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = env
		if err := cmd.Start(); err != nil {
			return utilsmsg.Wrap(utilsmsg.Internal, terminal.bin, "", err)
		}
		go func() {
			if err := cmd.Wait(); err != nil {
				slog.Warn("system update terminal",
					slog.String("terminal", terminal.bin),
					slog.String("error", err.Error()))
			}
		}()
		return nil
	}
	return errNoTerminal
}

func displayEnv(ctx context.Context) []string {
	env := os.Environ()
	if envValue(env, "DISPLAY") != "" || envValue(env, "WAYLAND_DISPLAY") != "" {
		return env
	}

	raw, err := exec.CommandContext(ctx, "systemctl", "--user", "show-environment").Output()
	if err != nil {
		return env
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || value == "" || strings.HasPrefix(value, "$'") {
			continue
		}
		if slices.Contains(displayEnvKeys, key) && envValue(env, key) == "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func envValue(env []string, key string) string {
	for _, one := range slices.Backward(env) {
		if value, ok := strings.CutPrefix(one, key+"="); ok {
			return value
		}
	}
	return ""
}
