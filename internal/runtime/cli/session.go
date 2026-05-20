package cli

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"

	"github.com/pardnchiu/agenvoy/internal/utils"
)

func Session(args []string) {
	sub := ""
	if len(args) > 0 {
		sub = strings.ToLower(strings.TrimSpace(args[0]))
	}
	name := ""
	if len(args) > 1 {
		name = strings.TrimSpace(args[1])
	}
	if sub == "" {
		sub = Pick("Select session action", []string{"new", "config"})
	}

	switch sub {
	case "new":
		NewSession(name)
	case "config":
		Config(name)
	default:
		fmt.Fprintf(os.Stderr, "Usage: agen session [new|config] [name]\n")
		os.Exit(1)
	}
}

func pickSession(label string) (sid string, hasSessions bool) {
	sessions := listSessions()
	if len(sessions) == 0 {
		return "", false
	}

	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].id < sessions[j].id
	})

	labels := make([]string, len(sessions)+1)
	for i, s := range sessions {
		short := utils.ShortenSessionID(s.id)
		entry := short
		if s.name != "" && s.name != s.id {
			entry = fmt.Sprintf("%s (%s)", short, s.name)
		}
		labels[i] = entry
	}
	labels[len(sessions)] = "exit"

	sel := promptui.Select{
		Label:        label,
		Items:        labels,
		HideSelected: true,
		Size:         10,
	}
	idx, _, err := sel.Run()
	if err != nil {
		slog.Error("promptui.Select.Run", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if idx == len(sessions) {
		os.Exit(0)
	}
	return sessions[idx].id, true
}
