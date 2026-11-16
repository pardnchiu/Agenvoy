package chatbot

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/pardnchiu/go-llm-router/core/claudeCode"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/compact"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

const NewUsage = "[Invalid] /new: input is required, usage: /new {input}"

const (
	menuModel     = "model"
	menuReasoning = "reasoning"
	menuPending   = "pending"
	menuStop      = "stop"
	menuCompact   = "compact"
)

const (
	compactYes = "Yes"
	compactNo  = "No"
)

type CommandReply struct {
	Text   string
	Menu   string
	Items  []string
	values map[string]string
}

type menuEntry struct {
	kind      string
	sessionID string
	values    map[string]string
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
	case "/pending":
		return taskMenu(sessionID, menuPending, interactive.ListResumablePending(sessionID)), true
	case "/stop":
		return taskMenu(sessionID, menuStop, interactive.ListOnlinePending(sessionID)), true
	case "/compact":
		if exec.IsRunning(sessionID) {
			return CommandReply{Text: "[Conflict] compact: a task is running, /stop it or wait"}, true
		}
		return CommandReply{Text: "Compact history? Redundant and meaningless exchanges are removed by LLM analysis", Menu: menuCompact, Items: []string{compactYes, compactNo}}, true
	}
	return CommandReply{}, false
}

func taskMenu(sessionID, kind string, hashes []string) CommandReply {
	var items []string
	values := map[string]string{}
	for _, hash := range hashes {
		info, ok := interactive.LoadPendingInfo(sessionID, hash)
		if !ok {
			continue
		}
		label := go_pkg_utils.TruncateString(hash, 8)
		if info.Objective != "" {
			label = go_pkg_utils.TruncateString(strings.Join(strings.Fields(info.Objective), " "), 12)
		}
		if info.HasQuestions {
			label = "❓ " + label
		}
		label = fmt.Sprintf("%d. %s", len(items)+1, label)
		if !info.UpdatedAt.IsZero() {
			label += " · " + info.UpdatedAt.Local().Format("01-02 15:04")
		}
		items = append(items, label)
		values[label] = hash
	}
	if kind == menuStop {
		if len(items) == 0 {
			return CommandReply{Text: "[Not found] running task: none to stop"}
		}
		return CommandReply{Text: fmt.Sprintf("%d running task(s)", len(items)), Menu: menuStop, Items: items, values: values}
	}
	if len(items) == 0 {
		return CommandReply{Text: "[Not found] pending task: none to resume"}
	}
	return CommandReply{Text: fmt.Sprintf("%d pending task(s)", len(items)), Menu: menuPending, Items: items, values: values}
}

func stopTask(sessionID, taskHash string) string {
	if !interactive.IsOnline(sessionID, taskHash) {
		return "[Not found] running task: it may have finished"
	}
	interactive.CancelTask(sessionID, taskHash)
	if !exec.Cancel(taskHash, runtime.ErrUserCanceled) {
		return "[Not found] running task: not running in this daemon"
	}
	return "Stop requested"
}

func resumePending(sessionID, taskHash string) string {
	info, ok := interactive.LoadPendingInfo(sessionID, taskHash)
	if !ok {
		return "[Not found] pending task: it may have been resumed or deleted"
	}
	if info.HasQuestions {
		if err := interactive.ReenqueueAskUser(sessionID, runtime.OriginOf(sessionID), taskHash); err != nil {
			return err.Error()
		}
		return ""
	}
	if !interactive.ClaimOnline(sessionID, taskHash) {
		return "[Conflict] pending task: running"
	}
	if !runtime.TriggerResume(sessionID, taskHash, nil) {
		interactive.ReleaseOnline(sessionID, taskHash)
		return "[Unavailable] pending task: no resume handler"
	}
	return ""
}

func MenuFallback(reply CommandReply) string {
	lines := []string{reply.Text}
	for _, item := range reply.Items {
		lines = append(lines, "• "+item)
	}
	switch reply.Menu {
	case menuModel:
		lines = append(lines, "Usage: /model {name}")
	case menuReasoning:
		lines = append(lines, "Usage: /reasoning {level}")
	}
	return strings.Join(lines, "\n")
}

func RegisterMenu(key, sessionID string, reply CommandReply) {
	menus.Store(key, menuEntry{kind: reply.Menu, sessionID: sessionID, values: reply.values})
}

func ApplyMenu(key, value string, send func(text string) (remove func())) bool {
	raw, ok := menus.LoadAndDelete(key)
	if !ok {
		return false
	}
	entry := raw.(menuEntry)
	var text string
	switch entry.kind {
	case menuPending, menuStop:
		hash, ok := entry.values[value]
		switch {
		case !ok:
			text = fmt.Sprintf("[Invalid] %s task: %q", entry.kind, value)
		case entry.kind == menuStop:
			text = stopTask(entry.sessionID, hash)
		default:
			text = resumePending(entry.sessionID, hash)
		}
	case menuCompact:
		text = runCompact(entry.sessionID, value, send)
	case menuReasoning:
		text = setReasoning(entry.sessionID, value)
	default:
		text = setModel(entry.sessionID, value)
	}
	if text != "" {
		send(text)
	}
	return true
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

func runCompact(sessionID, answer string, send func(text string) (remove func())) string {
	if answer != compactYes {
		return "Compact canceled"
	}
	if exec.IsRunning(sessionID) {
		return "[Conflict] compact: a task is running, /stop it or wait"
	}
	remove := send("Compacting...")
	removed, err := compact.SessionHistory(context.Background(), sessionID)
	remove()
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("Compacted: %d message(s) removed", removed)
}
