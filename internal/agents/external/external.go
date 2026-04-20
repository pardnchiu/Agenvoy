package external

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

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

func Run(ctx context.Context, agent, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	switch agent {
	case "codex":
		cmd = exec.CommandContext(ctx, "codex", "exec", prompt)
	case "copilot":
		cmd = exec.CommandContext(ctx, "gh", "copilot", "-p", prompt)
	case "claude":
		cmd = exec.CommandContext(ctx, "claude", "-p", prompt)
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

func RunParallel(ctx context.Context, agents []string, prompt string) []Result {
	ch := make(chan Result, len(agents))
	for _, a := range agents {
		go func(agent string) {
			out, err := Run(ctx, agent, prompt)
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
	if out, err := exec.Command("codex", "auth", "status").CombinedOutput(); err != nil {
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
