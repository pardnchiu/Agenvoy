package interactive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	goRuntime "runtime"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registInstallDependence() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "install_dependence",
		AlwaysAllow: false,
		Description: "Install a missing system binary cross-platform (TUI/CLI only). Skips if already in PATH. Never use run_command for package installs — sandbox blocks sudo. Language-level packages (pip/npm/cargo/gem) → output command for user to run manually.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"package": map[string]any{
					"type":        "string",
					"description": "Binary name to install (e.g. ffmpeg, yt-dlp). Single token, no flags or version pin.",
				},
			},
			"required": []string{"package"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Package string `json:"package"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json Unmarshal: %w", err)
			}

			pkg := strings.TrimSpace(params.Package)
			if pkg == "" {
				return "", fmt.Errorf("package is required")
			}
			if strings.ContainsAny(pkg, " \t\n\r;|&<>$`\"'\\") {
				return "", fmt.Errorf("package must be a single token without shell metacharacters")
			}

			if path, err := exec.LookPath(pkg); err == nil {
				return jsonString(map[string]any{
					"ok":                true,
					"name":              pkg,
					"already_installed": true,
					"path":              path,
				})
			}

			cmd, arg, via, err := buildCommand(pkg)
			if err != nil {
				return "", err
			}

			execCmd, execArgs, errPath, cleanup := stderrCapture(cmd, arg)
			defer cleanup()

			// * is TUI only, already excluded
			reply, err := runtime.Ask(ctx, runtime.Request{
				Kind:      runtime.KindExecProcess,
				SessionID: e.SessionID,
				ToolName:  "install_dependence",
				ExecProcess: &runtime.ExecPayload{
					Command: execCmd,
					Args:    execArgs,
				},
			})
			if err != nil {
				return "", fmt.Errorf("runtime Ask: %w", err)
			}
			if reply.Error != nil {
				return "", fmt.Errorf("runtime Ask: %w", reply.Error)
			}

			stderrMsg := readStderr(errPath)

			if reply.ExitCode != 0 {
				result := map[string]any{
					"ok":        false,
					"name":      pkg,
					"via":       via,
					"exit_code": reply.ExitCode,
				}
				if stderrMsg != "" {
					result["stderr"] = stderrMsg
				} else {
					result["message"] = fmt.Sprintf("install command %q exited with code %d (stderr capture unavailable)", cmd, reply.ExitCode)
				}
				return jsonString(result)
			}

			path, err := exec.LookPath(pkg)
			if err != nil {
				result := map[string]any{
					"ok":      false,
					"name":    pkg,
					"via":     via,
					"message": "install command succeeded but binary still not found in PATH",
				}
				if stderrMsg != "" {
					result["stderr"] = stderrMsg
				}
				return jsonString(result)
			}

			return jsonString(map[string]any{
				"ok":   true,
				"name": pkg,
				"via":  via,
				"path": path,
			})
		},
	})
}

func jsonString(dic map[string]any) (string, error) {
	raw, err := json.Marshal(dic)
	if err != nil {
		return "", fmt.Errorf("json Marshal: %w", err)
	}
	return string(raw), nil
}

func buildCommand(name string) (string, []string, string, error) {
	switch goRuntime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return "", nil, "", fmt.Errorf("brew not found in PATH; install Homebrew from https://brew.sh first")
		}
		return "brew", []string{"install", name}, "brew", nil

	case "linux":
		isRoot := os.Geteuid() == 0
		for _, pm := range []struct {
			binary string
			args   []string
		}{
			{"apt", []string{"apt", "install", "-y", name}},
			{"apt-get", []string{"apt-get", "install", "-y", name}},
			{"dnf", []string{"dnf", "install", "-y", name}},
			{"yum", []string{"yum", "install", "-y", name}},
			{"pacman", []string{"pacman", "-S", "--noconfirm", name}},
			{"apk", []string{"apk", "add", name}},
		} {
			if _, err := exec.LookPath(pm.binary); err != nil {
				continue
			}
			if isRoot {
				return pm.args[0], pm.args[1:], pm.binary, nil
			}
			if _, err := exec.LookPath("sudo"); err != nil {
				return "", nil, "", fmt.Errorf("sudo not found and current user is not root; cannot elevate for %s", pm.binary)
			}
			return "sudo", pm.args, pm.binary, nil
		}
		return "", nil, "", fmt.Errorf("no supported package manager (apt-get/dnf/yum/pacman/apk) found on this Linux system")

	default:
		return "", nil, "", fmt.Errorf("unsupported OS: %s (only darwin and linux are supported)", goRuntime.GOOS)
	}
}

func stderrCapture(cmdName string, cmdArgs []string) (string, []string, string, func()) {
	noop := func() {}

	if _, err := exec.LookPath("bash"); err != nil {
		return cmdName, cmdArgs, "", noop
	}

	f, err := os.CreateTemp("", "agen-install-err-*.log")
	if err != nil {
		return cmdName, cmdArgs, "", noop
	}
	errPath := f.Name()
	f.Close()

	tokens := make([]string, 0, len(cmdArgs)+1)
	tokens = append(tokens, shellQuoteSingle(cmdName))
	for _, a := range cmdArgs {
		tokens = append(tokens, shellQuoteSingle(a))
	}
	script := fmt.Sprintf("%s 2> >(tee %s >&2); exit ${PIPESTATUS[0]}",
		strings.Join(tokens, " "),
		shellQuoteSingle(errPath))

	cleanup := func() { os.Remove(errPath) }
	return "bash", []string{"-c", script}, errPath, cleanup
}

func readStderr(path string) string {
	if path == "" {
		return ""
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	result := strings.TrimSpace(string(raw))
	const max = 4096
	if len(result) > max {
		result = "...(truncated)\n" + result[len(result)-max:]
	}
	return result
}

func shellQuoteSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
