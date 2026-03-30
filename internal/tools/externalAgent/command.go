package externalAgent

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type agentResult struct {
	Agent  string
	Output string
	Err    error
}

func GetAgents() []string {
	var agents []string
	for _, name := range []string{"codex", "copilot", "claude"} {
		if os.Getenv("EXTERNAL_"+strings.ToUpper(name)) == "true" {
			agents = append(agents, name)
		}
	}
	return agents
}

func checkCLI(agent string) error {
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
