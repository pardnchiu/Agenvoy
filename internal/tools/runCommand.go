package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
	"github.com/pardnchiu/agenvoy/internal/utils/secret"
)

func registRunCommand() {
	toolregister.Regist(toolregister.Def{
		Name:        "run_command",
		Timeout:     configs.RUN_COMMAND_TIMEOUT,
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Description: fmt.Sprintf(`Runs one binary in the work directory and returns its combined stdout/stderr once it exits; networking is off unless network: true, and rm moves its targets to the trash instead of deleting them.
Use for "run this", "build it", "run the tests", "lint / format the code", and local git work such as commit, branch or merge.
A watcher (--watch, chokidar, npm run sass/build/dev), a job likely over %d minutes, or "run it in the background" / "tell me when it's done" → background_command; cat / head / tail → read_files; ls / find / grep / rg → find_files; a URL → fetch_page or http_request; %s; opening a file or link in an app → open_file.`, configs.BACKGROUND_THRESHOLD_MIN, systemPackageRoute),
		Parameters: map[string]any{
			"type":       "object",
			"properties": commandProperties(),
			"required":   []string{"argv"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params commandParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			return runCommand(ctx, e, params.Argv, params.WritePaths, params.Network, params.unsandboxed(), nil)
		},
	})
}

type commandParams struct {
	Argv       []string `json:"argv"`
	Sandbox    *bool    `json:"sandbox"`
	Network    bool     `json:"network"`
	WritePaths []string `json:"write_paths"`
}

func (p commandParams) unsandboxed() bool {
	return p.Sandbox != nil && !*p.Sandbox
}

func commandProperties() map[string]any {
	return map[string]any{
		"argv": map[string]any{
			"type":        "array",
			"description": "The command as an argv array; the first item is a bare binary name, not a path: ['git','status'], ['go','test','./...'].\n- Pipe, redirect, glob, ~ or && → ['sh','-c','<full command>']; otherwise call the binary directly. Inside sh -c, command names are literal (no $VAR or $(...) in command position); rm and sudo are refused there.\n- Script file, including a multi-line script you write first → ['sh','<file>'], or sh <file> inside sh -c; ./<file> is refused like any path in command position.\n- cd is refused → absolute paths, or ['sh','-c','cd <dir> && <command>'].\n- Root → ['sudo','<binary>', ...]: no sudo flags, no nested sudo or su; runs outside the sandbox.\n- Read-only inspection (git status / log / diff, du, which, docker ps, --version) goes in its own call, never chained with && to a command that writes, so it matches the read-only allowlist and skips confirmation.\n- A capability rather than an exact binary → first ['sh','-c','command -v <every candidate>'], which prints only the installed ones, instead of guessing one name per call.\n- A key or token in a file or the keychain → read and use it in the same command, never read it out first, since secrets in tool output come back as ********: ['sh','-c','set -a; . ./.env; set +a; curl -H \"Authorization: Bearer $API_TOKEN\" <url>'], or $(security find-generic-password -s agenvoy -a <KEY> -w) as an argument.",
			"items":       map[string]any{"type": "string"},
			"minItems":    1,
		},
		"network": map[string]any{
			"type":        "boolean",
			"description": "true when the command has to reach a host: git clone / fetch / pull / push, npm / pnpm / yarn / pip install, go mod download, cargo fetch, curl, wget. Without it these fail to resolve the host; leave it out for anything that only touches local files.",
		},
		"write_paths": map[string]any{
			"type":        "array",
			"description": "Absolute paths outside $HOME this command writes to, e.g. ['/usr/local'] for a system install; paths under $HOME are always writable and need not be listed. The sandbox only allows writes under $HOME, so a missing entry fails with a permission error that looks like file ownership and is not one: list the path here instead of running chown or chmod. A path still denied while listed is root-owned → argv ['sudo', ...]. Ignored when sandbox is false or argv starts with sudo.",
			"items":       map[string]any{"type": "string"},
		},
		"sandbox": map[string]any{
			"type":        "boolean",
			"description": "false runs the command outside the sandbox with the user's full permissions; network and write_paths are then ignored. Use it only after the sandbox blocked this exact command with an error network or write_paths cannot fix (diskutil -69863, operation not permitted on a system service); a root-owned target needs argv ['sudo', ...] instead. Homebrew (brew install / upgrade / uninstall) runs its own sandbox, which cannot nest: pass false, never sudo.",
			"default":     true,
		},
	}
}

func deniedCommandErr(binary string) error {
	return utilsmsg.New(utilsmsg.Denied, "run_command", "%s is on denied_command and can never run; it cannot be approved and retrying changes nothing", binary)
}

var errSudoInShell = utilsmsg.New(utilsmsg.Denied, "sudo", "cannot elevate inside sh -c; call run_command with argv ['sudo', '<binary>', ...<args>] instead, which asks the user for the system password and runs outside the sandbox. For %s", systemPackageRoute)

func sudoArgv(argv []string) ([]string, error) {
	if len(argv) < 2 {
		return nil, utilsmsg.New(utilsmsg.Invalid, "sudo", "needs the command to elevate, e.g. ['sudo', 'rm', '/usr/local/bin/tool']")
	}
	target := argv[1]
	if strings.HasPrefix(target, "-") {
		return nil, utilsmsg.New(utilsmsg.Invalid, "sudo", "flags are not accepted (%q); pass ['sudo', '<binary>', ...<args>]", target)
	}
	binary := filepath.Base(target)
	if binary != target {
		return nil, utilsmsg.New(utilsmsg.Invalid, "sudo", "%q must be a bare command name (%q), not a path", target, binary)
	}
	if binary == "sudo" || binary == "su" {
		return nil, utilsmsg.New(utilsmsg.Invalid, "sudo", "%s cannot be nested under sudo", binary)
	}
	if slices.Contains(filesystem.DeniedCommand, binary) {
		return nil, deniedCommandErr(binary)
	}
	return append([]string{"sudo", "-n"}, argv[1:]...), nil
}

var errOpenCommand = utilsmsg.New(utilsmsg.Denied, "open", "the sandbox has no route to the app-launch service (error -54); call open_file with the file path or URL instead")

const systemPackageRoute = "a system package → the package manager named on the Host OS line: argv ['sudo', '<manager>', ...] (e.g. ['sudo', 'apt-get', 'install', '-y', '<pkg>']), or ['brew', 'install', '<pkg>'] with sandbox: false and never sudo; only when Host OS names none, find it with command -v"

func runCommand(ctx context.Context, e *tooltypes.Executor, argv, writePaths []string, network, unsandboxed bool, background *backgroundSpec) (string, error) {
	if len(argv) == 0 {
		return "", utilsmsg.New(utilsmsg.Invalid, "argv", "requires a non-empty array, e.g. [\"git\", \"status\"]")
	}

	if masked := secret.FindMasked(strings.Join(argv, "\n")); masked != "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "argv", "contains %q, a redacted value from a tool result, not the real text; read the original inside this command (e.g. . ./.env, security find-generic-password) instead", masked)
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
			elevated, err := sudoArgv(argv)
			if err != nil {
				return "", err
			}
			argv = elevated
			unsandboxed = true
		}
		if binary == "open" && runtime.GOOS == "darwin" {
			return "", errOpenCommand
		}
		if binary == "rm" {
			return moveToTrash(ctx, e, argv[1:])
		}
	}

	if background == nil {
		if err := checkWatchArgs(argv, e.WorkDir, 0, map[string]bool{}); err != nil {
			return "", err
		}
	}

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
	if unsandboxed {
		sandboxOpt = &go_pkg_sandbox.Option{AllowAll: true}
	}

	if background != nil {
		return startBackgroundCommand(ctx, e, argv, binds, sandboxOpt, background)
	}

	ctx, cancel := context.WithTimeout(ctx, configs.RUN_COMMAND_TIMEOUT)
	defer cancel()

	cmd, err := go_pkg_sandbox.Wrap(ctx, argv[0], argv[1:], e.WorkDir, sandboxOpt)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_sandbox.Wrap", "", err)
	}
	relaySudoCancel(cmd, argv)

	sink := &progressWriter{send: tooltypes.Progress(ctx), redact: secret.Stream()}
	cmd.Stdout = sink
	cmd.Stderr = sink

	err = go_pkg_sandbox.Run(cmd)
	output := sink.text()
	if err != nil {
		return fmt.Sprintf("%s\nError: %s%s", output, err.Error(), sandboxWriteHint(output, binds, unsandboxed)), nil
	}

	if strings.TrimSpace(output) == "" {
		return "[exit 0] the command succeeded and printed nothing", nil
	}
	return output, nil
}

const progressInterval = 500 * time.Millisecond

func relaySudoCancel(cmd *exec.Cmd, argv []string) {
	if argv[0] != "sudo" {
		return
	}
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
}

type progressWriter struct {
	mu      sync.Mutex
	buf     strings.Builder
	pending strings.Builder
	last    time.Time
	send    func(string)
	redact  func(string) string
	carry   string
}

func (w *progressWriter) Write(raw []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(raw)
	data := w.carry + string(raw)
	cut := strings.LastIndexAny(data, "\r\n") + 1
	w.carry = data[cut:]
	w.pending.WriteString(w.redact(data[:cut]))

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

const sandboxEscalateAdvice = `

[agenvoy] the output reports a permission error; the sandbox may have caused it. Leaving the sandbox is the last resort, not the default retry:
- a connection or auth failure (e.g. Permission denied (publickey), could not resolve host) → network: true, or report the credential problem; sandbox: false does not fix it
- a write → a path under the work directory, or write_paths for the path that failed
- the task genuinely needs a system socket, device or service, or a root-owned target → sandbox: false or argv ['sudo', ...]; the user approves it with the system password
Otherwise report what was blocked and stop.`

var permissionMarkers = []string{
	"not writable", "permission denied", "operation not permitted",
	"read-only file system", "eacces",
}

var absPathPattern = regexp.MustCompile(`(^|[\s"'(])(/[A-Za-z0-9._@+\-/]{2,})`)

func sandboxWriteHint(output string, bound []string, unsandboxed bool) string {
	if unsandboxed {
		return ""
	}
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
		return sandboxEscalateAdvice
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

func RestrictedRunCommand(toolName, toolArgs string) []string {
	if toolName != "run_command" && toolName != "background_command" {
		return nil
	}
	var p struct {
		Argv    []string `json:"argv"`
		Sandbox *bool    `json:"sandbox"`
	}
	if json.Unmarshal([]byte(toolArgs), &p) != nil {
		return nil
	}
	if len(p.Argv) > 0 && p.Argv[0] == "sudo" {
		return []string{"sudo: " + strings.Join(p.Argv[1:], " ")}
	}
	if p.Sandbox == nil || *p.Sandbox {
		return nil
	}
	return []string{"outside sandbox: " + strings.Join(p.Argv, " ")}
}
