package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registRunCommand() {
	toolregister.Regist(toolregister.Def{
		Name:        "run_command",
		Timeout:     configs.RUN_COMMAND_TIMEOUT,
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Description: fmt.Sprintf(`Runs one binary in the work directory, waits for it to exit, returns its combined stdout/stderr. Networking is off by default and opened per call with network: true, which always raises a confirmation: git clone / fetch / pull / push, npm or pip install, go mod download, curl and wget each need it set or they fail to resolve a host.
Use for 跑一下 / 執行 / build / test / lint / format and local git work — one binary and its arguments.
A watcher never exits and hangs the call (--watch, chokidar, npm run sass/build/dev) → run the one-shot build.
cat / head / tail → read_files; ls / find / grep / rg → find_files; a URL → fetch_page or http_request; %s; open in an app → open_file.
Inspection only (git status / log / diff, du, which, docker ps, git config --list, --version) runs without asking, matched against a read-only allowlist by binary plus up to two subcommands; anything else raises a confirmation, so never chain one behind && with a command that writes.`, systemPackageRoute()),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"argv": map[string]any{
					"type":        "array",
					"description": "The command as an argv array — ['git','status'], ['go','test','./...']. A single pipeline or redirect needs ['sh','-c','<full command>']; without a shell metacharacter (| && > * ~) it is called directly, never wrapped in sh -c. A multi-line script goes in a file you write first, then run that file. When the request names a capability rather than an exact command, resolve which binary is installed before running one: a single ['sh','-c','command -v <every candidate>'] prints only those that exist. Guessing the most common name costs one round trip per guess and the failure reads as 'command not found', not as 'wrong binary'.",
					"items":       map[string]any{"type": "string"},
					"minItems":    1,
				},
				"network": map[string]any{
					"type":        "boolean",
					"description": "Set true when the command has to reach a host — git clone / fetch / pull / push, npm or pnpm or yarn install, pip install, go mod download, cargo fetch, curl. The sandbox keeps networking off until this is set, and setting it always raises a confirmation, so leave it out for anything that only touches local files.",
				},
				"write_paths": map[string]any{
					"type":        "array",
					"description": "Absolute paths outside $HOME this command has to write to — ['/usr/local'] for a system install. The sandbox only allows writes under $HOME, so a command touching anything else fails with a permission error that looks like a file ownership problem and is not one. Each path needs the user's approval on this call. Paths under $HOME need not be listed.",
					"items":       map[string]any{"type": "string"},
				},
			},
			"required": []string{"argv"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Argv       []string `json:"argv"`
				Network    bool     `json:"network"`
				WritePaths []string `json:"write_paths"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			return runCommand(ctx, e, params.Argv, params.WritePaths, params.Network)
		},
	})
}

func deniedCommandErr(binary string) error {
	return utilsmsg.New(utilsmsg.Denied, "run_command", "%s is on denied_command and can never run; it cannot be approved and retrying changes nothing", binary)
}

func sudoCommandErr() error {
	return utilsmsg.New(utilsmsg.Denied, "sudo", "cannot elevate inside run_command: the sandbox blocks it. Re-run the command without sudo and declare write_paths with the absolute paths outside $HOME it writes to; that raises the sudo confirm, where the user approves them with the system password. For %s", systemPackageRoute())
}

func openCommandErr() error {
	return utilsmsg.New(utilsmsg.Denied, "open", "the sandbox has no route to the app-launch service (error -54); call open_file with the file path or URL instead")
}

func systemPackageRoute() string {
	return "installing or removing a system package → pkg_manage"
}

func runCommand(ctx context.Context, e *tooltypes.Executor, argv, writePaths []string, network bool) (string, error) {
	if len(argv) == 0 {
		return "", utilsmsg.New(utilsmsg.Invalid, "argv", "requires a non-empty array, e.g. [\"git\", \"status\"]")
	}

	binary := filepath.Base(argv[0])
	denied := filesystem.DeniedCommand

	if binary != argv[0] {
		return "", utilsmsg.New(utilsmsg.Invalid, "run_command", "%q must be a bare command name (%q), not a path", argv[0], binary)
	}

	if (binary == "sh" || binary == "bash") && len(argv) >= 3 && argv[1] == "-c" {
		if slices.Contains(denied, binary) {
			return "", deniedCommandErr(binary)
		}
		if strings.TrimSpace(argv[2]) == "" {
			return "", utilsmsg.New(utilsmsg.Invalid, binary+" -c", "requires a non-empty command string")
		}
		if err := validateShellScript(argv[2], denied); err != nil {
			return "", err
		}
	} else {
		if binary == "cd" {
			return "", utilsmsg.New(utilsmsg.Invalid, "cd", "the work directory is set by the user and cannot be changed here; pass absolute paths, or ['sh','-c','cd <dir> && <command>'] for a single command")
		}
		if slices.Contains(denied, binary) {
			return "", deniedCommandErr(binary)
		}
		if binary == "sudo" {
			return "", sudoCommandErr()
		}
		if binary == "open" && runtime.GOOS == "darwin" {
			return "", openCommandErr()
		}
		if binary == "rm" {
			return moveToTrash(ctx, e, argv[1:])
		}
	}

	if err := checkWatchArgs(argv, e.WorkDir, 0, map[string]bool{}); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, configs.RUN_COMMAND_TIMEOUT)
	defer cancel()

	binds, err := boundary.WriteBinds(e.SessionID, e.WorkDir, writePaths)
	if err != nil {
		return "", err
	}
	sandboxOpt := &go_pkg_sandbox.Option{Network: go_pkg_sandbox.NetworkDeny}
	if network {
		sandboxOpt.Network = go_pkg_sandbox.NetworkAllow
	}
	if len(binds) > 0 {
		sandboxOpt.MinimalBinds = &go_pkg_sandbox.BindSpec{
			WriteScope: go_pkg_sandbox.WriteHome,
			ReadWrite:  binds,
		}
	}

	cmd, err := go_pkg_sandbox.Wrap(ctx, argv[0], argv[1:], e.WorkDir, sandboxOpt)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_sandbox.Wrap", "", err)
	}

	sink := &progressWriter{send: tooltypes.Progress(ctx)}
	cmd.Stdout = sink
	cmd.Stderr = sink

	err = cmd.Run()
	output := sink.text()
	if err != nil {
		return fmt.Sprintf("%s\nError: %s%s", output, err.Error(), sandboxWriteHint(output, binds)), nil
	}

	if strings.TrimSpace(output) == "" {
		return "[exit 0] the command succeeded and printed nothing", nil
	}
	return output, nil
}

const progressInterval = 500 * time.Millisecond

type progressWriter struct {
	mu      sync.Mutex
	buf     strings.Builder
	pending strings.Builder
	last    time.Time
	send    func(string)
}

func (w *progressWriter) Write(raw []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(raw)
	w.pending.Write(raw)

	if time.Since(w.last) < progressInterval {
		return len(raw), nil
	}
	line := lastLine(w.pending.String())
	w.pending.Reset()
	w.last = time.Now()
	if line != "" {
		w.send(line)
	}
	return len(raw), nil
}

func (w *progressWriter) text() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func lastLine(chunk string) string {
	for line := range strings.SplitSeq(strings.TrimRight(chunk, "\r\n"), "\n") {
		chunk = line
	}
	return strings.TrimSpace(go_pkg_utils.TruncateString(chunk, 200))
}

const sandboxWriteAdvice = `

[agenvoy] the sandbox only allows writes under %s, which is why these read as unwritable: %s. They are writable outside the sandbox, so ownership is not the problem — do not run chown or chmod and do not ask the user to. Re-run this exact command with write_paths: [%s] and the user gets one prompt to approve those paths.`

var permissionMarkers = []string{
	"not writable", "permission denied", "operation not permitted",
	"read-only file system", "eacces",
}

var absPathPattern = regexp.MustCompile(`(^|[\s"'(])(/[A-Za-z0-9._@+\-/]{2,})`)

func sandboxWriteHint(output string, bound []string) string {
	lower := strings.ToLower(output)
	if !slices.ContainsFunc(permissionMarkers, func(m string) bool { return strings.Contains(lower, m) }) {
		return ""
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	seen := map[string]bool{}
	var list []string
	for _, match := range absPathPattern.FindAllStringSubmatch(output, -1) {
		one := strings.TrimRight(match[2], ".,:;)")
		if one == "" || seen[one] || strings.HasPrefix(one, home) {
			continue
		}
		if slices.ContainsFunc(bound, func(b string) bool { return one == b || strings.HasPrefix(one, b+"/") }) {
			continue
		}
		if isSystemBinary(one) {
			continue
		}
		seen[one] = true
		list = append(list, one)
	}

	list = shortestRoots(list)
	if len(list) == 0 {
		return ""
	}

	quoted := make([]string, 0, len(list))
	for _, one := range list {
		quoted = append(quoted, strconv.Quote(one))
	}
	return fmt.Sprintf(sandboxWriteAdvice, home, strings.Join(list, ", "), strings.Join(quoted, ", "))
}

var systemBinDirs = []string{"/usr/bin/", "/bin/", "/usr/sbin/", "/sbin/", "/usr/libexec/"}

func isSystemBinary(path string) bool {
	return slices.ContainsFunc(systemBinDirs, func(d string) bool { return strings.HasPrefix(path, d) })
}

func stableRoot(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return path
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		return path
	}
	return "/" + parts[0] + "/" + parts[1]
}

func shortestRoots(list []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, one := range list {
		root := stableRoot(one)
		if seen[root] {
			continue
		}
		seen[root] = true
		out = append(out, root)
	}
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}
