package tui

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/agents/exec/compact"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	compactPrefix = "compact:"
	resetPrefix   = "reset:"
)

type CompactConfirm struct {
	id  string
	yes bool
}

type CompactDone struct {
	id      string
	removed int
	err     error
}

func (t TUI) commandCompactReset(tab int) (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		slog.Debug("tui: no active session")
		return t, nil, true
	}

	label := utils.ShortenSessionID(sid)
	width := t.width
	if width < 20 {
		width = 80
	}
	popup := &Popup{
		kind:  popupSingleSelect,
		title: "/compact",
		tabs:  []string{"summary", "compact", "reset"},
		onConfirm: func(chosen string) any {
			if mode, ok := strings.CutPrefix(chosen, resetPrefix); ok {
				return ResetSessionConfirm1{id: sid, mode: mode}
			}
			return CompactConfirm{id: sid, yes: chosen == compactPrefix+"yes"}
		},
	}
	popup.onTab = func(p *Popup) tea.Cmd {
		p.cursor = 0
		p.readOnly = p.tabIdx == 0
		if p.readOnly {
			p.subtitle = fmt.Sprintf("summary for %s", label)
			p.options = summaryLines(sid, max(width-16, 30))
			p.values = nil
			return nil
		}
		if p.tabIdx == 2 {
			p.subtitle = fmt.Sprintf("reset history for %s  summary: regenerate then keep  all: also wipe the summary", label)
			p.options = []string{"No", "Yes  summary first, keep it", "Yes  reset all (summary too)"}
			p.values = []string{resetPrefix + "no", resetPrefix + "summary", resetPrefix + "all"}
			return nil
		}
		p.subtitle = fmt.Sprintf("compact history for %s  redundant and meaningless exchanges are removed by LLM analysis", label)
		p.options = []string{"No", "Yes"}
		p.values = []string{compactPrefix + "no", compactPrefix + "yes"}
		return nil
	}
	popup.tabIdx = tab
	popup.onTab(popup)
	t.popup = popup
	return t, nil, true
}

var summaryFieldOrder = []string{"current_discussion", "key_decisions", "past_discussions", "topic", "last_discussed", "description", "perspectives", "direction"}

func summaryKeys(dic map[string]any) []string {
	keys := make([]string, 0, len(dic))
	for _, key := range summaryFieldOrder {
		if _, ok := dic[key]; ok {
			keys = append(keys, key)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(dic)) {
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	return keys
}

func summaryLines(sid string, width int) []string {
	dic, err := go_pkg_filesystem.ReadJSON[map[string]any](filesystem.SummaryPath(sid))
	if err != nil || len(dic) == 0 {
		return []string{"no summary yet"}
	}

	var lines []string
	add := func(indent int, first, text string) {
		pad := strings.Repeat(" ", indent)
		for i, line := range strings.Split(wrapText(text, width-indent-len(first)), "\n") {
			if i == 0 {
				lines = append(lines, pad+whiteStyle.Render(first+line))
				continue
			}
			lines = append(lines, pad+strings.Repeat(" ", len(first))+whiteStyle.Render(line))
		}
	}
	var render func(value any, indent int)
	render = func(value any, indent int) {
		switch v := value.(type) {
		case map[string]any:
			for _, key := range summaryKeys(v) {
				name := strings.ReplaceAll(key, "_", " ")
				switch child := v[key].(type) {
				case map[string]any, []any:
					add(indent, "", name+":")
					render(child, indent+2)
				default:
					add(indent, "", fmt.Sprintf("%s: %v", name, child))
				}
			}
		case []any:
			for i, item := range v {
				if _, ok := item.(map[string]any); ok {
					if i > 0 {
						lines = append(lines, "")
					}
					render(item, indent)
					continue
				}
				add(indent, "- ", fmt.Sprint(item))
			}
		default:
			add(indent, "", fmt.Sprint(v))
		}
	}
	for _, key := range summaryKeys(dic) {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, skillStyle.Render(strings.ReplaceAll(key, "_", " ")))
		render(dic[key], 2)
	}
	return lines
}

func (t TUI) runCompact(sid string) (TUI, tea.Cmd) {
	t.running = true
	t.runStartedAt = time.Now()
	t.runTarget = utils.ShortenSessionID(sid)
	t.activity = "compacting history..."

	slog.Debug("tui: compacting history", slog.String("session", sid))
	return t, tea.Batch(
		t.spinner.Tick,
		func() tea.Msg {
			ctx := context.Background()
			removed, err := compact.SessionHistory(ctx, sid)
			return CompactDone{id: sid, removed: removed, err: err}
		},
	)
}

func (t TUI) finishCompact(msg CompactDone) (TUI, tea.Cmd) {
	t.running = false
	t.activity = ""
	t.runTarget = ""

	if msg.err != nil {
		return t, notice(msgError(fmt.Sprintf("compact failed: %v", msg.err)) + "\n")
	}

	t.tokens = 0
	t.lastIn = 0
	t.lastContext = 0
	t.lastOut = 0
	t.lastCacheRead = 0
	t.lastCacheCreate = 0

	slog.Debug("tui: compact finished",
		slog.String("session", msg.id),
		slog.Int("removed", msg.removed))

	seq := []tea.Cmd{
		tea.ClearScreen,
		tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.lineStatus, t.currentSessionID)),
	}
	tail := loadSessionTail(msg.id, t.width, false)
	seq = append(seq, tail...)
	return t, tea.Sequence(seq...)
}
