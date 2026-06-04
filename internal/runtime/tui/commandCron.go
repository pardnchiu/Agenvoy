package tui

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type CronAction struct {
	action string
}

func (t TUI) commandCron(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "add":
			return t.commandCronAdd()
		case "remove":
			return t.commandCronRemove()
		case "edit":
			return t.commandCronEdit()
		}
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Cron",
		options: []string{"add", "remove", "edit"},
		values:  []string{"add", "remove", "edit"},
		cursor:  0,
		onConfirm: func(chosen string) any {
			return CronAction{action: chosen}
		},
	}
	return t, nil, true
}

func listCronEntries() []runtime.CronEntry {
	crons, err := runtime.LoadCrons()
	if err != nil {
		return nil
	}
	sort.Slice(crons, func(i, j int) bool {
		if crons[i].Skill != crons[j].Skill {
			return crons[i].Skill < crons[j].Skill
		}
		return crons[i].Expression < crons[j].Expression
	})
	return crons
}

func cronOptions(crons []runtime.CronEntry) (labels, values []string) {
	labels = make([]string, len(crons))
	values = make([]string, len(crons))
	for i, c := range crons {
		labels[i] = c.Expression + "  " + c.Skill
		values[i] = c.Skill
	}
	return labels, values
}

func (t TUI) dispatchAgent(content string) (TUI, tea.Cmd) {
	if content == "" {
		return t, nil
	}
	if len(agents.Registry().Entries) == 0 {
		return t, tea.Println(warnStyle.Render("⎯ no model configured · /model global add") + "\n")
	}
	t = t.recordInputHistory(content)
	t.running = true
	t.runStartedAt = time.Now()
	t.runTarget = targetSession(content, t.currentSessionID)

	go runExec(t.ctx, content, false, t.cwd, t.currentSessionID, "", t.mode == webMode)

	return t, tea.Batch(
		tea.Println(messageBlock(content)),
		t.spinner.Tick,
	)
}
