package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (t TUI) commandSchedule(parts []string) (TUI, tea.Cmd, bool) {
	name := strings.TrimPrefix(parts[0], "/sched-")
	if name == "" {
		return t, tea.Println(errorStyle.Render("[!] scheduler skill name required") + "\n"), true
	}
	if !filesystem.ScheduleSkillExists(name) {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] scheduler skill %q not found", name)) + "\n"), true
	}
	body, err := filesystem.ScheduleSkillBody(name)
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] read scheduler skill: %v", err)) + "\n"), true
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] scheduler skill %q is empty", name)) + "\n"), true
	}

	extra := strings.TrimSpace(strings.Join(parts[1:], " "))
	preamble := fmt.Sprintf("[執行已存在 scheduler skill: %s · 此為手動 trigger，不是建立新 schedule]\n依下方 SKILL body instructions 立即執行並輸出結果。**禁止** activate `scheduler-skill-creator`、**禁止** 跑 `init_scheduler_skill.py`、**禁止** add_task／add_cron——skill 已存在、已綁時間，本次只執行 body。\n\n---\n", name)
	prompt := preamble + body
	if extra != "" {
		prompt = prompt + "\n\n---\n附加指令：" + extra
	}
	next, cmd := t.dispatchAgent(prompt)
	return next, cmd, true
}
