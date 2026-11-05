package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/runtime/ipc"
	"github.com/pardnchiu/agenvoy/internal/sudo"
)

var sudoWord = regexp.MustCompile(`(^|[\s;&|(])sudo(\s|$)`)

type shellPassword struct {
	command string
	workDir string
}

type shellSudo struct {
	command  string
	workDir  string
	password string
}

type shellDone struct {
	command  string
	workDir  string
	output   string
	exitCode int
	err      error
}

func runShell(command, workDir, password string) {
	ctx, cancel := context.WithTimeout(context.Background(), configs.RUN_COMMAND_TIMEOUT)
	defer cancel()
	send(agentExec{cancel: func(error) { cancel() }})

	if sudoWord.MatchString(command) {
		if password == "" && !sudo.Cached(ctx) {
			send(shellPassword{command: command, workDir: workDir})
			return
		}
		if password != "" {
			if err := sudo.Verify(ctx, password); err != nil {
				send(shellDone{command: command, workDir: workDir, err: err})
				return
			}
		}
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := osexec.CommandContext(ctx, shell, "-c", command)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()

	if errors.Is(ctx.Err(), context.Canceled) {
		send(agentExecDone{err: context.Canceled})
		return
	}

	done := shellDone{
		command: command,
		workDir: workDir,
		output:  go_pkg_utils.TruncateString(strings.TrimRight(string(out), "\n"), configs.MAX_PENDING_RESULT_BYTES),
	}
	var exitErr *osexec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		done.exitCode = exitErr.ExitCode()
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		done.err = fmt.Errorf("[Timeout] %s: exceeded %s", command, configs.RUN_COMMAND_TIMEOUT)
	default:
		done.err = fmt.Errorf("[Internal] %s: %w", command, err)
	}
	send(done)
}

func (t TUI) openShellPassword(msg shellPassword) (tea.Model, tea.Cmd) {
	t.running = false
	t.cancelExec = nil
	t.popup = &Popup{
		kind:     popupSecret,
		title:    "System password:",
		subtitle: msg.command,
		input:    newPopupInput("", false),
		onConfirm: func(value string) any {
			return shellSudo{command: msg.command, workDir: msg.workDir, password: value}
		},
	}
	return t, nil
}

func (t TUI) startShellSudo(msg shellSudo) (tea.Model, tea.Cmd) {
	if msg.password == "" {
		return t, notice(msgWarn("[Canceled] sudo: no password entered") + "\n")
	}
	t.running = true
	t.runStartedAt = time.Now()
	go runShell(msg.command, msg.workDir, msg.password)
	return t, t.spinner.Tick
}

func (t TUI) handleShellDone(msg shellDone) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		t.running = false
		t.cancelExec = nil
		return t, notice(msgError(msg.err.Error()) + "\n")
	}

	output := msg.output
	if output == "" {
		output = "(no output)"
	}
	input := strings.NewReplacer(
		"{{.WorkDir}}", msg.workDir,
		"{{.Command}}", msg.command,
		"{{.ExitCode}}", strconv.Itoa(msg.exitCode),
		"{{.Output}}", output,
	).Replace(strings.TrimSpace(configs.InjectShellRun))

	go runExec(ipc.Frame{Type: ipc.FrameRun, SessionID: t.currentSessionID, Rayload: &ipc.Payload{Input: input, WorkDir: msg.workDir, AllowAll: t.allowAll}})

	header := hintStyle.Render(fmt.Sprintf("exit %d", msg.exitCode))
	return t, tea.Println(header + "\n" + whiteStyle.Render(output) + "\n")
}
