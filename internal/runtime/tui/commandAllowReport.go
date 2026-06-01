package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

const (
	reportSkillName = "report_error_to_developer"
	reportCronExpr  = "0 9,15,21 * * *"
)

const reportSkillBody = `---
name: report_error_to_developer
description: 每天 09:00 / 15:00 / 21:00 收集過去 6 小時的 daemon WARN/ERROR 並上傳給開發者。
---

# Report Errors To Developer

呼叫 report_error 工具，參數 h=6。

該工具會自行掃描過去 6 小時的 WARN/ERROR：有錯誤就上傳給開發者，沒有就略過。

完成後只回一行簡短結果（例如「已上傳 N 行」或「過去 6 小時無錯誤」），不要貼出完整 log。
`

type AllowReportAction struct{ action string }

type AllowReportConfirm struct {
	action string
	yes    bool
}

func (t TUI) commandAllowReport(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "enable", "disable":
			action := parts[1]
			return t, func() tea.Msg { return AllowReportAction{action: action} }, true
		}
	}

	cursor := 0
	if reportScheduleEnabled() {
		cursor = 1
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Report errors to developer",
		options: []string{"enable", "disable"},
		values:  []string{"enable", "disable"},
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return AllowReportAction{action: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) openAllowReportConfirm(action string) (TUI, tea.Cmd) {
	var title, subtitle, yesLabel string
	switch action {
	case "enable":
		title = "Expose WARN/ERROR logs to the developer?"
		subtitle = "Daemon WARN/ERROR lines from the last 6h will be uploaded to report.agenvoy.com every day at 09:00 / 15:00 / 21:00."
		yesLabel = "Yes, enable"
	case "disable":
		title = "Stop sending error reports to the developer?"
		subtitle = "The daily 09:00 / 15:00 / 21:00 schedule will be removed."
		yesLabel = "Yes, disable"
	default:
		return t, nil
	}
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    title,
		subtitle: subtitle,
		options:  []string{"No", yesLabel},
		values:   []string{"no", "yes"},
		cursor:   0,
		onConfirm: func(chosen string) any {
			return AllowReportConfirm{action: action, yes: chosen == "yes"}
		},
	}
	return t, nil
}

func reportScheduleEnabled() bool {
	crons, err := runtime.LoadCrons()
	if err != nil {
		return false
	}
	for _, c := range crons {
		if c.Skill == reportSkillName {
			return true
		}
	}
	return false
}

func (t TUI) runAllowReportEnable() (TUI, tea.Cmd) {
	if err := go_pkg_filesystem.CheckDir(filesystem.ScheduleSkillDir(reportSkillName), true); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] allow-report: %v", err)) + "\n")
	}
	if err := go_pkg_filesystem.WriteFile(filesystem.ScheduleSkillPath(reportSkillName), reportSkillBody, 0644); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] allow-report: %v", err)) + "\n")
	}
	if _, err := runtime.RemoveCron(reportSkillName); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] allow-report: %v", err)) + "\n")
	}
	entry := runtime.CronEntry{
		Expression: reportCronExpr,
		SessionID:  strings.TrimSpace(t.currentSessionID),
		Skill:      reportSkillName,
	}
	if err := runtime.AppendCron(entry); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] allow-report: %v", err)) + "\n")
	}
	return t, tea.Println(hintStyle.Render("⎯ allow-report enabled · daily 09:00 / 15:00 / 21:00 · daemon reloading") + "\n")
}

func (t TUI) runAllowReportDisable() (TUI, tea.Cmd) {
	if _, err := runtime.RemoveCron(reportSkillName); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] allow-report: %v", err)) + "\n")
	}
	if err := skill.TrashSchedule(context.Background(), reportSkillName); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] allow-report: %v", err)) + "\n")
	}
	return t, tea.Println(hintStyle.Render("⎯ allow-report disabled · schedule removed") + "\n")
}
