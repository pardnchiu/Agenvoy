package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/runtime/kuradb"
	"github.com/pardnchiu/agenvoy/internal/session/config"
)

const kuradbInstallURL = "https://agenvoy.com/scripts/kuradb/install.sh"

type KuradbAction struct {
	action string
}

type KuradbKeySubmit struct {
	token string
}

type KuradbDone struct {
	action string
	err    error
}

func (t TUI) commandKuradb(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "enable", "disable", "update":
			action := parts[1]
			return t, func() tea.Msg { return KuradbAction{action: action} }, true
		}
	}

	enabled := false
	if cfg, err := config.Load(); err == nil && cfg != nil {
		enabled = cfg.KuradbEnabled
	}
	options := []string{"enable", "disable"}
	cursor := 0
	if enabled {
		options = append(options, "update")
		cursor = 1
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "KuraDB",
		options: options,
		values:  options,
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return KuradbAction{action: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) openKuradbKeyPrompt() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "KuraDB · OPENAI_API_KEY",
		subtitle: "required for embedding (text-embedding-3-small) · Enter to submit · Esc to cancel",
		onConfirm: func(value string) any {
			return KuradbKeySubmit{token: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func runKuradbEnableExec() tea.Cmd {
	script := fmt.Sprintf(`set -e
curl -fsSL %s | bash
kura add agenvoy 2>/dev/null || true
`, kuradbInstallURL)

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = os.Environ()
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return KuradbDone{action: "enable", err: fmt.Errorf("install script: %w", err)}
		}
		if !kuradb.IsInstalled() {
			return KuradbDone{action: "enable", err: fmt.Errorf("kura binary not at %s after install", kuradb.BinaryPath)}
		}
		cfg, err := config.Load()
		if err != nil {
			return KuradbDone{action: "enable", err: fmt.Errorf("session.Load: %w", err)}
		}
		cfg.KuradbEnabled = true
		if err := config.Save(cfg); err != nil {
			return KuradbDone{action: "enable", err: fmt.Errorf("session.Save: %w", err)}
		}
		return KuradbDone{action: "enable"}
	})
}

func runKuradbUpdateExec() tea.Cmd {
	if cfg, err := config.Load(); err == nil && cfg != nil {
		cfg.KuradbEnabled = false
		if err := config.Save(cfg); err != nil {
			return func() tea.Msg {
				return KuradbDone{action: "update", err: fmt.Errorf("session.Save(stop): %w", err)}
			}
		}
	}

	script := fmt.Sprintf(`set -e
curl -fsSL %s | bash
kura add agenvoy 2>/dev/null || true
`, kuradbInstallURL)

	cmd := exec.Command("bash", "-c", script)
	cmd.Env = os.Environ()
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			if kuradb.IsInstalled() {
				if cfg, lerr := config.Load(); lerr == nil && cfg != nil {
					cfg.KuradbEnabled = true
					_ = config.Save(cfg)
				}
			}
			return KuradbDone{action: "update", err: fmt.Errorf("install script: %w", err)}
		}
		if !kuradb.IsInstalled() {
			return KuradbDone{action: "update", err: fmt.Errorf("kura binary not at %s after install", kuradb.BinaryPath)}
		}
		cfg, err := config.Load()
		if err != nil {
			return KuradbDone{action: "update", err: fmt.Errorf("session.Load: %w", err)}
		}
		cfg.KuradbEnabled = true
		if err := config.Save(cfg); err != nil {
			return KuradbDone{action: "update", err: fmt.Errorf("session.Save: %w", err)}
		}
		return KuradbDone{action: "update"}
	})
}

func runKuradbDisableExec() tea.Cmd {
	cmd := exec.Command("sudo", "rm", "-f", kuradb.BinaryPath)
	cmd.Env = os.Environ()
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return KuradbDone{action: "disable", err: fmt.Errorf("rm %s: %w", kuradb.BinaryPath, err)}
		}
		cfg, err := config.Load()
		if err != nil {
			return KuradbDone{action: "disable", err: fmt.Errorf("session.Load: %w", err)}
		}
		cfg.KuradbEnabled = false
		if err := config.Save(cfg); err != nil {
			return KuradbDone{action: "disable", err: fmt.Errorf("session.Save: %w", err)}
		}
		return KuradbDone{action: "disable"}
	})
}
