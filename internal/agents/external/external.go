package external

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runCodex(ctx context.Context, prompt string, readOnly bool) (string, error) {
	outFile := filepath.Join(os.TempDir(), fmt.Sprintf("agenvoy-codex-%d.txt", time.Now().UnixNano()))
	defer os.Remove(outFile)

	args := []string{"exec", "--output-last-message", outFile, "--skip-git-repo-check"}
	if !readOnly {
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, "codex", args...)
	combined, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(combined))
		if trimmed != "" {
			return "", fmt.Errorf("%s: %s", err.Error(), trimmed)
		}
		return "", err
	}

	data, readErr := os.ReadFile(outFile)
	if readErr != nil {
		return "", fmt.Errorf("read codex output: %w", readErr)
	}
	output := strings.TrimSpace(string(data))
	if output == "" {
		return "", fmt.Errorf("empty response from codex")
	}
	return output, nil
}

type Result struct {
	Agent  string
	Output string
	Err    error
}

func Agents() []string {
	var agents []string
	for _, name := range []string{"codex", "copilot", "claude"} {
		if os.Getenv("EXTERNAL_"+strings.ToUpper(name)) == "true" {
			agents = append(agents, name)
		}
	}
	return agents
}

func Check(agent string) error {
	switch agent {
	case "codex":
		return checkCodex()
	case "copilot":
		return checkCopilot()
	case "claude":
		return checkClaude()
	default:
		return fmt.Errorf("%s not supported", agent)
	}
}

func CheckAgents() ([]string, map[string]error) {
	var agents []string
	errors := make(map[string]error)
	for _, a := range Agents() {
		if err := Check(a); err != nil {
			errors[a] = err
		} else {
			agents = append(agents, a)
		}
	}
	return agents, errors
}

func Run(ctx context.Context, agent, prompt string, readOnly bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if agent == "codex" {
		return runCodex(ctx, prompt, readOnly)
	}

	var cmd *exec.Cmd
	switch agent {
	case "copilot":
		args := []string{"copilot", "-s", "-p", prompt}
		if !readOnly {
			args = append(args, "--allow-all-tools", "--allow-all-paths", "--allow-all-urls")
		}
		cmd = exec.CommandContext(ctx, "gh", args...)
	case "claude":
		args := []string{"-p"}
		if readOnly {
			args = append(args, "--disallowedTools=Edit,Write,NotebookEdit")
		} else {
			args = append(args, "--permission-mode", "acceptEdits")
		}
		args = append(args, prompt)
		cmd = exec.CommandContext(ctx, "claude", args...)
	default:
		return "", fmt.Errorf("%s not supported", agent)
	}

	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		if output != "" {
			return "", fmt.Errorf("%s: %s", err.Error(), output)
		}
		return "", err
	}
	if output == "" {
		return "", fmt.Errorf("empty response from %s", agent)
	}
	return output, nil
}

func RunParallel(ctx context.Context, agents []string, prompt string, readOnly bool) []Result {
	ch := make(chan Result, len(agents))
	for _, a := range agents {
		go func(agent string) {
			out, err := Run(ctx, agent, prompt, readOnly)
			ch <- Result{Agent: agent, Output: out, Err: err}
		}(a)
	}

	results := make([]Result, 0, len(agents))
	for range agents {
		results = append(results, <-ch)
	}
	return results
}

func checkCodex() error {
	if _, err := exec.LookPath("codex"); err != nil {
		return fmt.Errorf("please install first: npm install -g @openai/codex")
	}
	if out, err := exec.Command("codex", "login", "status").CombinedOutput(); err != nil {
		return fmt.Errorf("please login first: codex login - %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func checkCopilot() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("please install first: gh")
	}
	if _, err := exec.Command("gh", "copilot", "--version").CombinedOutput(); err != nil {
		return fmt.Errorf("please install/login first: gh extension install github/gh-copilot or gh auth login")
	}
	return nil
}

func checkClaude() error {
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("please install first: npm install -g @anthropic-ai/claude-code")
	}
	if _, err := exec.Command("claude", "--version").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run claude, please check your installation and login first")
	}
	return nil
}
