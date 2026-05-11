package tui

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func (t TUI) webMode() (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return t, tea.Println(errorStyle.Render("⎯ /mode: no active session")), true
	}

	if err := ensureDefaultIndex(sid); err != nil {
		return t, tea.Println(errorStyle.Render("⎯ /mode: " + err.Error())), true
	}

	port := go_pkg_utils.GetWithDefault("PORT", "17989")
	url := "http://localhost:" + port + "/" + sid + "/"

	if err := openBrowser(url); err != nil {
		t.mode = webMode
		return t, tea.Println(
			warnStyle.Render("⎯ web mode · ") +
				hintStyle.Render("open browser manually: "+url+" ("+err.Error()+")"),
		), true
	}

	t.mode = webMode
	return t, nil, true
}

func ensureDefaultIndex(sid string) error {
	pageDir := filesystem.PagePath(sid)
	if err := go_pkg_filesystem.CheckDir(pageDir, true); err != nil {
		return err
	}
	indexPath := filepath.Join(pageDir, "index.html")
	if go_pkg_filesystem_reader.Exists(indexPath) {
		return nil
	}
	return go_pkg_filesystem.WriteFile(indexPath, configs.WebmodeHTML, 0644)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return &unsupportedOSError{os: runtime.GOOS}
	}
	return cmd.Start()
}

type unsupportedOSError struct {
	os string
}

func (e *unsupportedOSError) Error() string {
	return "unsupported os: " + e.os
}
