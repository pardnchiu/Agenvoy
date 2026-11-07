package webapp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
	"github.com/pardnchiu/agenvoy/page"
)

func Install(ctx context.Context) (string, error) {
	if utils.IsWSL() {
		return installWindows(ctx)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os.UserHomeDir", "", err)
	}

	entry := filepath.Join(home, ".local", "share", "applications", "agenvoy.desktop")
	if go_pkg_filesystem_reader.Exists(entry) {
		return entry, nil
	}

	binary := ""
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			binary = path
			break
		}
	}
	if binary == "" {
		return "", utilsmsg.New(utilsmsg.NotFound, "chrome binary", "none in PATH")
	}

	raw, err := page.FS.ReadFile("public/icon-512.png")
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "page.FS.ReadFile", "", err)
	}

	iconDir := filepath.Join(home, ".local", "share", "icons", "hicolor", "512x512", "apps")
	for _, dir := range []string{iconDir, filepath.Dir(entry)} {
		if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", "", err)
		}
	}

	if err := go_pkg_filesystem.WriteFile(filepath.Join(iconDir, "agenvoy.png"), string(raw), 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteFile", "", err)
	}

	if err := go_pkg_filesystem.WriteFile(entry, desktopEntry(binary), 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteFile", "", err)
	}
	return entry, nil
}

func desktopEntry(binary string) string {
	return `[Desktop Entry]
Type=Application
Name=` + configs.APP_NAME + `
Comment=Agenvoy web console
Exec=` + binary + ` --app=` + appURL() + `
Icon=agenvoy
Terminal=false
Categories=Development;Utility;
X-Agenvoy-Version=` + appVersion() + `
`
}

func installWindows(ctx context.Context) (string, error) {
	desktop, err := powershell(ctx, "[Environment]::GetFolderPath('Desktop')")
	if err != nil {
		return "", err
	}
	if desktop == "" {
		return "", utilsmsg.New(utilsmsg.Upstream, "windows Desktop", "resolved empty")
	}

	link := desktop + `\` + configs.APP_NAME + ".lnk"
	linkDir, err := wslPath(ctx, "-u", desktop)
	if err != nil {
		return "", err
	}
	if go_pkg_filesystem_reader.Exists(filepath.Join(linkDir, configs.APP_NAME+".lnk")) {
		return link, nil
	}

	chrome := utils.WSLChromePath()
	if chrome == "" {
		return "", utilsmsg.New(utilsmsg.NotFound, "chrome.exe", "none under /mnt/c")
	}
	target, err := wslPath(ctx, "-w", chrome)
	if err != nil {
		return "", err
	}

	localApp, err := powershell(ctx, "$env:LOCALAPPDATA")
	if err != nil {
		return "", err
	}
	if localApp == "" {
		return "", utilsmsg.New(utilsmsg.Upstream, "windows LOCALAPPDATA", "resolved empty")
	}

	iconWin := localApp + `\` + configs.APP_NAME + `\icon.ico`
	iconDir, err := wslPath(ctx, "-u", localApp+`\`+configs.APP_NAME)
	if err != nil {
		return "", err
	}
	if err := go_pkg_filesystem.CheckDir(iconDir, true); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", "", err)
	}

	raw, err := page.FS.ReadFile("public/icon.ico")
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "page.FS.ReadFile", "", err)
	}
	if err := go_pkg_filesystem.WriteFile(filepath.Join(iconDir, "icon.ico"), string(raw), 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteFile", "", err)
	}

	script := strings.Join([]string{
		"$s = (New-Object -ComObject WScript.Shell).CreateShortcut(" + psQuote(link) + ")",
		"$s.TargetPath = " + psQuote(target),
		"$s.Arguments = " + psQuote("--app="+windowsURL()),
		"$s.IconLocation = " + psQuote(iconWin),
		"$s.Description = " + psQuote(configs.APP_NAME+" "+appVersion()),
		"$s.Save()",
	}, "; ")
	if _, err := powershell(ctx, script); err != nil {
		return "", err
	}
	return link, nil
}

func windowsURL() string {
	return "http://localhost:" + filesystem.Port
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func wslPath(ctx context.Context, flag, value string) (string, error) {
	raw, err := exec.CommandContext(ctx, "wslpath", flag, value).Output()
	if err != nil {
		return "", utilsmsg.New(utilsmsg.Internal, "wslpath", "%s: %w%s", flag, err, stderrOf(err))
	}
	return strings.TrimSpace(string(raw)), nil
}

func powershell(ctx context.Context, script string) (string, error) {
	bin, err := exec.LookPath("powershell.exe")
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.NotFound, "exec.LookPath", "powershell.exe", err)
	}
	raw, err := exec.CommandContext(ctx, bin, "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return "", utilsmsg.New(utilsmsg.Internal, "powershell.exe", "%w%s", err, stderrOf(err))
	}
	return strings.TrimSpace(string(raw)), nil
}

func stderrOf(err error) string {
	var exit *exec.ExitError
	if !errors.As(err, &exit) || len(exit.Stderr) == 0 {
		return ""
	}
	raw := exit.Stderr
	if len(raw) > 8<<10 {
		raw = raw[:8<<10]
	}
	return ": " + strings.TrimSpace(string(raw))
}
