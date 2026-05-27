package cli

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type sessionInfo struct {
	id   string
	name string
}

func listSessions() []sessionInfo {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return nil
	}
	out := make([]sessionInfo, 0, len(dirs))
	for _, d := range dirs {
		sid := d.Name
		if strings.HasPrefix(sid, "temp-") {
			continue
		}
		name, _ := session.GetBot(sid)
		out = append(out, sessionInfo{id: sid, name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

const pickSessionNew = "__new__"

func pickSession(label string) string {
	sessions := listSessions()

	labels := make([]string, 0, len(sessions)+2)
	values := make([]string, 0, len(sessions)+2)

	for _, s := range sessions {
		short := utils.ShortenSessionID(s.id)
		entry := short
		if s.name != "" && s.name != s.id {
			entry = fmt.Sprintf("%s (%s)", short, s.name)
		}
		labels = append(labels, entry)
		values = append(values, s.id)
	}
	labels = append(labels, "(new session)")
	values = append(values, pickSessionNew)
	labels = append(labels, "exit")
	values = append(values, "")

	sel := promptui.Select{
		Label:        label,
		Items:        labels,
		HideSelected: true,
		Size:         min(8, len(labels)),
	}
	idx, _, err := sel.Run()
	if err != nil {
		slog.Error("promptui.Select.Run", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if values[idx] == "" {
		os.Exit(0)
	}
	return values[idx]
}
