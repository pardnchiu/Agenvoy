package tui

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/runtime/daemon"
)

const roleTimeout = 10 * time.Second

type roleEntry struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type RolePick struct {
	name string
}

type RoleLoaded struct {
	name    string
	content string
	err     error
}

type RoleTitleSubmit struct {
	origin string
	title  string
}

type RoleBodySubmit struct {
	origin string
	title  string
	body   string
}

type RoleSaved struct {
	name string
	err  error
}

func (t TUI) commandRole() (TUI, tea.Cmd, bool) {
	next, cmd := t.openLoading("/role", rolePopup)
	return next, cmd, true
}

func rolePopup() *Popup {
	ctx, cancel := context.WithTimeout(context.Background(), roleTimeout)
	defer cancel()

	out, err := daemon.Get[map[string][]roleEntry](ctx, "/v1/roles", nil)
	if err != nil {
		return errorPopup("/role", fmt.Sprintf("role list: %v", err))
	}
	names := make([]string, 0, len(out["roles"]))
	for _, one := range out["roles"] {
		if one.Name != "" {
			names = append(names, one.Name)
		}
	}
	sort.Strings(names)

	options := []string{"New"}
	values := []string{""}
	if len(names) > 0 {
		options = append(options, "")
		values = append(values, "")
	}

	return &Popup{
		kind:     popupSingleSelect,
		title:    "/role",
		subtitle: "pick one to edit",
		options:  append(options, names...),
		values:   append(values, names...),
		onConfirm: func(chosen string) any {
			return RolePick{name: chosen}
		},
	}
}

func (t TUI) runRolePick(msg RolePick) (TUI, tea.Cmd) {
	if msg.name == "" {
		t.roleBodyDraft = ""
		return t.showRoleTitlePopup("", "")
	}

	name := msg.name
	return t, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), roleTimeout)
		defer cancel()

		out, err := daemon.Get[roleEntry](ctx, "/v1/role/"+url.PathEscape(name), nil)
		if err != nil {
			return RoleLoaded{name: name, err: err}
		}
		return RoleLoaded{name: out.Name, content: out.Content}
	}
}

func (t TUI) runRoleLoaded(msg RoleLoaded) (TUI, tea.Cmd) {
	if msg.err != nil {
		return t, notice(msgError(fmt.Sprintf("role read %s: %v", msg.name, msg.err)) + "\n")
	}

	t.roleBodyDraft = msg.content
	return t.showRoleTitlePopup(msg.name, msg.name)
}

func (t TUI) showRoleTitlePopup(origin, title string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:  popupText,
		title: "role title",
		input: newPopupInput(title, false),
		onConfirm: func(value string) any {
			return RoleTitleSubmit{origin: origin, title: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) runRoleTitleSubmit(msg RoleTitleSubmit) (TUI, tea.Cmd) {
	if msg.title == "" {
		t.roleBodyDraft = ""
		return t, notice(msgError("role title required") + "\n")
	}
	return t.showRoleBodyPopup(msg.origin, msg.title)
}

func (t TUI) showRoleBodyPopup(origin, title string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:      popupText,
		title:     fmt.Sprintf("role description (%s)", title),
		multiline: true,
		input:     newPopupInput(t.roleBodyDraft, true),
		onConfirm: func(value string) any {
			return RoleBodySubmit{origin: origin, title: title, body: value}
		},
	}
	t.roleBodyDraft = ""
	return t, nil
}

func (t TUI) roleSaveCmd(msg RoleBodySubmit) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), roleTimeout)
		defer cancel()

		var out roleEntry
		var err error
		if msg.origin == "" {
			out, err = daemon.Post[roleEntry](ctx, "/v1/role", map[string]any{"name": msg.title, "content": msg.body})
		} else {
			body := map[string]any{"name": msg.origin, "content": msg.body}
			if msg.title != msg.origin {
				body["rename"] = msg.title
			}
			out, err = daemon.Patch[roleEntry](ctx, "/v1/role", body)
		}
		if err != nil {
			return RoleSaved{name: msg.title, err: err}
		}
		return RoleSaved{name: out.Name}
	}
}

func (t TUI) runRoleSaved(msg RoleSaved) (TUI, tea.Cmd) {
	if msg.err != nil {
		return t, notice(msgError(fmt.Sprintf("role save %s: %v", msg.name, msg.err)) + "\n")
	}
	slog.Debug("role saved", slog.String("name", msg.name))
	return t, nil
}
