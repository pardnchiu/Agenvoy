package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"golang.org/x/term"

	"github.com/pardnchiu/agenvoy/internal/pending"
)

func NewPending(ctx context.Context) {
	pending.Active.Store(true)
	defer pending.Active.Store(false)

	reader := bufio.NewReader(os.Stdin)

	for {
		for {
			id, req, ok := pending.PickNext()
			if !ok {
				break
			}

			process(id, req, reader)
		}

		select {
		case <-ctx.Done():
			return
		case <-pending.Notify:
		}
	}
}

func process(id string, req pending.Request, reader *bufio.Reader) {
	if req.Ctx != nil {
		if err := req.Ctx.Err(); err != nil {
			pending.Resolve(id, pending.Reply{Err: err})
			return
		}
	}

	switch req.Kind {
	case pending.KindToolConfirm:
		pending.Resolve(id, runToolConfirm(req))
	case pending.KindAskUser:
		pending.Resolve(id, runAskUser(req, reader))
	default:
		pending.Resolve(id, pending.Reply{Err: fmt.Errorf("unknown pending kind: %s", req.Kind)})
	}
}

func runToolConfirm(req pending.Request) pending.Reply {
	if req.ToolName == "run_command" {
		writeStdoutLine(ansiYellow + fmt.Sprintf("[$] [%s] %s", time.Now().Format("15:04:05"), printLogCommand(req.ToolArgs)) + ansiReset)
		dollarLinePending.Store(true)
	}
	prompt := promptui.Select{
		Label:        fmt.Sprintf("Run %s?", req.ToolName),
		Items:        []string{"Yes", "Skip", "Stop"},
		Size:         3,
		HideSelected: true,
	}
	idx, _, err := prompt.Run()
	switch {
	case err != nil || idx == 2:
		return pending.Reply{Approve: false, Err: fmt.Errorf("user stopped")}
	case idx == 1:
		return pending.Reply{Approve: false, Skip: true}
	default:
		return pending.Reply{Approve: true}
	}
}

func runAskUser(req pending.Request, reader *bufio.Reader) pending.Reply {
	if req.AskUser == nil || len(req.AskUser.Questions) == 0 {
		return pending.Reply{Err: fmt.Errorf("ask_user with no questions")}
	}

	answers := make([]any, 0, len(req.AskUser.Questions))
	for i, q := range req.AskUser.Questions {
		question := strings.TrimSpace(q.Question)
		if question == "" {
			return pending.Reply{Err: fmt.Errorf("question #%d is empty", i+1)}
		}

		switch {
		case len(q.Options) == 0 && q.Secret:
			ans, err := askSecretInput(question)
			if err != nil {
				return pending.Reply{Err: err}
			}
			answers = append(answers, ans)

		case len(q.Options) == 0:
			ans, err := askInput(reader, question)
			if err != nil {
				return pending.Reply{Err: err}
			}
			answers = append(answers, ans)

		case q.MultiSelect:
			ans, err := askMultiSelect(reader, question, q.Options, i+1)
			if err != nil {
				return pending.Reply{Err: err}
			}
			answers = append(answers, ans)

		default:
			ans, err := askSingleSelect(question, q.Options)
			if err != nil {
				return pending.Reply{Err: err}
			}
			answers = append(answers, ans)
		}
	}

	return pending.Reply{Answers: answers}
}

func askInput(reader *bufio.Reader, question string) (string, error) {
	if _, err := fmt.Fprintf(os.Stdout, "[?] %s\n> ", question); err != nil {
		return "", fmt.Errorf("write prompt: %w", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func askSecretInput(question string) (string, error) {
	if _, err := fmt.Fprintf(os.Stdout, "[?] %s: ", question); err != nil {
		return "", fmt.Errorf("write prompt: %w", err)
	}
	raw, readErr := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stdout)
	if readErr != nil {
		return "", fmt.Errorf("term.ReadPassword: %w", readErr)
	}
	return strings.TrimSpace(string(raw)), nil
}

func askSingleSelect(question string, options []string) (string, error) {
	prompt := promptui.Select{
		Label:        question,
		Items:        options,
		Size:         len(options),
		HideSelected: false,
	}
	_, chosen, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("promptui.Select: %w", err)
	}
	return chosen, nil
}

func askMultiSelect(reader *bufio.Reader, question string, options []string, qIdx int) ([]string, error) {
	if _, err := fmt.Fprintf(os.Stdout, "[?] %s (multi-select, comma-separated indices)\n", question); err != nil {
		return nil, fmt.Errorf("write prompt: %w", err)
	}
	for i, opt := range options {
		if _, err := fmt.Fprintf(os.Stdout, "  %d) %s\n", i+1, opt); err != nil {
			return nil, fmt.Errorf("write prompt: %w", err)
		}
	}
	if _, err := fmt.Fprint(os.Stdout, "> "); err != nil {
		return nil, fmt.Errorf("write prompt: %w", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	line = strings.TrimSpace(line)

	seen := make(map[int]bool, len(options))
	selected := make([]string, 0, len(options))
	for _, tok := range strings.Split(line, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		idx, err := strconv.Atoi(tok)
		if err != nil || idx < 1 || idx > len(options) {
			return nil, fmt.Errorf("invalid multi-select input %q for question #%d: expected comma-separated integers in 1..%d", line, qIdx, len(options))
		}
		if seen[idx] {
			continue
		}
		seen[idx] = true
		selected = append(selected, options[idx-1])
	}
	return selected, nil
}
