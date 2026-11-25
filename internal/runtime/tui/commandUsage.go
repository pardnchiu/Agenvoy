package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/configs"
	usagelog "github.com/pardnchiu/agenvoy/internal/session/usage"
)

func (t TUI) commandUsage() (TUI, tea.Cmd, bool) {
	sessionID := strings.TrimSpace(t.currentSessionID)
	next, cmd := t.openLoading("/usage", func() *Popup { return usagePopup(sessionID) })
	return next, cmd, true
}

func usagePopup(sessionID string) *Popup {
	now := time.Now()

	sessions := make([]map[string]usagelog.ModelUsage, len(configs.USAGE_PERIODS))
	totals := make([]map[string]usagelog.ModelUsage, len(configs.USAGE_PERIODS))
	labels := make([]string, len(configs.USAGE_PERIODS))
	for i, period := range configs.USAGE_PERIODS {
		labels[i] = period.Label
		if sessionID != "" {
			summary, err := usagelog.Usage(sessionID, period.Days, now)
			if err != nil {
				return errorPopup("/usage", fmt.Sprintf("usage: %v", err))
			}
			sessions[i] = summary
		}
		total, err := usagelog.Total(period.Days, now)
		if err != nil {
			return errorPopup("/usage", fmt.Sprintf("usage: %v", err))
		}
		totals[i] = total
	}

	nameWidth := max(usageNameWidth(sessions), usageNameWidth(totals))
	popup := &Popup{
		kind:       popupSingleSelect,
		title:      "/usage",
		subtitle:   hintStyle.Render("  input(cache hit%)/output"),
		maxVisible: usageMaxVisible,
		readOnly:   true,
		tabs:       labels,
	}
	popup.onTab = func(p *Popup) tea.Cmd {
		fillUsageOptions(p, sessionID != "", sessions, totals, nameWidth)
		return nil
	}
	popup.onTab(popup)

	return popup
}

const usageMaxVisible = 16

func usageNameWidth(summaries []map[string]usagelog.ModelUsage) int {
	width := len("model")
	for _, summary := range summaries {
		for model, one := range summary {
			if one.Input == 0 && one.Output == 0 {
				continue
			}
			if len(model) > width {
				width = len(model)
			}
		}
	}
	return width
}

func fillUsageOptions(p *Popup, hasSession bool, sessions, totals []map[string]usagelog.ModelUsage, nameWidth int) {
	idx := p.tabIdx
	if idx < 0 || idx >= len(totals) {
		idx = 0
	}

	options := []string{"session"}
	tails := []string{""}
	if hasSession {
		rows, rowTails := usageRows(sessions[idx], nameWidth)
		options = append(options, rows...)
		tails = append(tails, rowTails...)
	} else {
		options = append(options, "  no active session")
		tails = append(tails, "")
	}

	options = append(options, "", "global")
	tails = append(tails, "", "")
	rows, rowTails := usageRows(totals[idx], nameWidth)
	options = append(options, rows...)
	tails = append(tails, rowTails...)

	p.options = options
	p.optionTail = tails
	p.values = nil
	p.cursor = 0
}

func usageRows(summary map[string]usagelog.ModelUsage, nameWidth int) ([]string, []string) {
	models := make([]string, 0, len(summary))
	for model, one := range summary {
		if one.Input == 0 && one.Output == 0 {
			continue
		}
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		left, right := summary[models[i]].Input, summary[models[j]].Input
		if left == right {
			return models[i] < models[j]
		}
		return left > right
	})

	options := make([]string, 0, len(models))
	tails := make([]string, 0, len(models))
	for _, model := range models {
		options = append(options, fmt.Sprintf("  %-*s", nameWidth, model))
		tails = append(tails, formatUsageCell(summary[model]))
	}
	if len(options) == 0 {
		options = append(options, "  no usage")
		tails = append(tails, "")
	}
	return options, tails
}

func formatUsageCell(u usagelog.ModelUsage) string {
	hitPct := 0.0
	if total := u.Input + u.Hit; total > 0 {
		hitPct = float64(u.Hit) / float64(total) * 100
	}
	rounded := int(hitPct + 0.5)
	var pct string
	switch {
	case rounded <= 0:
		pct = "--%"
	case rounded >= 100:
		pct = "00%"
	default:
		pct = fmt.Sprintf("%2d%%", rounded)
	}
	return fmt.Sprintf("%s(%s)/%s", color(u.Input), pct, color(u.Output))
}

func color(value uint64) string {
	plain := fmt.Sprintf("%7s", formatUsageCount(value))
	switch {
	case value >= 1_000_000_000:
		return errorStyle.Render(plain)
	case value >= 1_000_000:
		return systemStyle.Render(plain)
	case value >= 1_000:
		return okayStyle.Render(plain)
	default:
		return plain
	}
}

func formatUsageCount(value uint64) string {
	units := []struct {
		threshold uint64
		suffix    string
	}{
		{threshold: 1_000_000_000, suffix: "B"},
		{threshold: 1_000_000, suffix: "M"},
		{threshold: 1_000, suffix: "K"},
	}
	for _, unit := range units {
		if value >= unit.threshold {
			return fmt.Sprintf("%.2f%s", float64(value)/float64(unit.threshold), unit.suffix)
		}
	}
	return fmt.Sprintf("%d", value)
}
