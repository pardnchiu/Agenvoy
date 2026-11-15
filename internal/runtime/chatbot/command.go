package chatbot

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/pardnchiu/go-llm-router/core/claudeCode"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
)

const NewUsage = "[Invalid] /new: input is required, usage: /new {input}"

const (
	menuModel     = "model"
	menuReasoning = "reasoning"
)

type CommandReply struct {
	Text  string
	Menu  string
	Items []string
}

type menuEntry struct {
	kind      string
	sessionID string
}

var menus sync.Map

func SessionCommand(sessionID, content string) (CommandReply, bool) {
	head, rest := content, ""
	if i := strings.IndexFunc(content, unicode.IsSpace); i >= 0 {
		head, rest = content[:i], strings.TrimSpace(content[i:])
	}
	cmd, _, _ := strings.Cut(head, "@")

	switch cmd {
	case "/model":
		if rest == "" {
			current, _ := configbot.GetModel(sessionID)
			return CommandReply{Text: "Model (current: " + current + ")", Menu: menuModel, Items: availableModels()}, true
		}
		return CommandReply{Text: setModel(sessionID, rest)}, true
	case "/reasoning":
		if rest == "" {
			_, current := configbot.GetModel(sessionID)
			return CommandReply{Text: "Reasoning (current: " + current + ")", Menu: menuReasoning, Items: configbot.ReasoningLevels()}, true
		}
		return CommandReply{Text: setReasoning(sessionID, rest)}, true
	}
	return CommandReply{}, false
}

func MenuFallback(reply CommandReply) string {
	usage := "/model {name}"
	if reply.Menu == menuReasoning {
		usage = "/reasoning {level}"
	}
	lines := []string{reply.Text}
	for _, item := range reply.Items {
		lines = append(lines, "• "+item)
	}
	return strings.Join(append(lines, "Usage: "+usage), "\n")
}

func RegisterMenu(key, sessionID string, reply CommandReply) {
	menus.Store(key, menuEntry{kind: reply.Menu, sessionID: sessionID})
}

func ApplyMenu(key, value string) (string, bool) {
	raw, ok := menus.LoadAndDelete(key)
	if !ok {
		return "", false
	}
	entry := raw.(menuEntry)
	if entry.kind == menuReasoning {
		return setReasoning(entry.sessionID, value), true
	}
	return setModel(entry.sessionID, value), true
}

func availableModels() []string {
	list := []string{configbot.DefaultModel}
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return list
	}
	skipClaudeCode := !agenttypes.ClaudeCodeEnabled()
	for _, m := range cfg.Models {
		if skipClaudeCode && claudeCode.Is(m.Name) {
			continue
		}
		list = append(list, m.Name)
	}
	return list
}

func setModel(sessionID, name string) string {
	normalized := config.NormalizeModel(strings.TrimSpace(name))
	if !slices.Contains(availableModels(), normalized) {
		return fmt.Sprintf("[Not found] model: %q", name)
	}
	configbot.SetModel(sessionID, normalized, "")
	return "Model set to " + normalized
}

func setReasoning(sessionID, level string) string {
	levels := configbot.ReasoningLevels()
	level = strings.ToLower(strings.TrimSpace(level))
	if !slices.Contains(levels, level) {
		return fmt.Sprintf("[Invalid] reasoning: %q, expected %s", level, strings.Join(levels, "/"))
	}
	configbot.SetModel(sessionID, "", level)
	return "Reasoning set to " + level
}
