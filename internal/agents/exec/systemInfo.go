package exec

import (
	"context"
	"os/exec"
	goruntime "runtime"
	"slices"
	"strings"
	"sync"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type systemInfo struct {
	os   string
	wsl  bool
	home string
}

var (
	systemInfoOnce sync.Once
	systemInfoMeta systemInfo
)

const (
	winHomeProbeTimeout = 3 * time.Second
	winHomeFallback     = "/mnt/<drive>/Users/<WinUser>"
	osVersionTimeout    = 3 * time.Second
)

var packageManagers = []string{"apt-get", "dnf", "yum", "pacman", "apk", "zypper", "brew"}

var brewPaths = []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew", "/home/linuxbrew/.linuxbrew/bin/brew"}

func getSystemInfo() systemInfo {
	systemInfoOnce.Do(func() {
		platform := []string{goruntime.GOOS + "/" + goruntime.GOARCH}
		if tag := utils.WSLTag(); tag != "" {
			platform = append(platform, tag)
			systemInfoMeta.wsl = true
			systemInfoMeta.home = windowsHome()
		}

		desc := goruntime.GOOS
		if name := osName(); name != "" {
			desc = name
		}
		desc += " (" + strings.Join(platform, ", ") + ")"
		if pm := packageManager(); pm != "" {
			desc += "; package manager: " + pm
		}
		systemInfoMeta.os = desc
	})
	return systemInfoMeta
}

func osName() string {
	switch goruntime.GOOS {
	case "darwin":
		ctx, cancel := context.WithTimeout(context.Background(), osVersionTimeout)
		defer cancel()
		out, err := exec.CommandContext(ctx, "sw_vers", "-productVersion").Output()
		if err != nil {
			return "macOS"
		}
		return "macOS " + strings.TrimSpace(string(out))
	case "linux":
		raw, err := go_pkg_filesystem.ReadText("/etc/os-release")
		if err != nil {
			return ""
		}
		for line := range strings.SplitSeq(raw, "\n") {
			if value, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
				return strings.Trim(strings.TrimSpace(value), `"'`)
			}
		}
	}
	return ""
}

func packageManager() string {
	for _, name := range packageManagers {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	if slices.ContainsFunc(brewPaths, go_pkg_filesystem_reader.IsFile) {
		return "brew"
	}
	return ""
}

func windowsHome() string {
	if path := winHomeFromInterop(); path != "" {
		return path
	}
	return winHomeFromMount()
}

func winHomeFromInterop() string {
	cmdBin, err := exec.LookPath("cmd.exe")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), winHomeProbeTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, cmdBin, "/c", "echo %USERPROFILE%").Output()
	if err != nil {
		return ""
	}
	winPath := strings.TrimSpace(string(out))
	if winPath == "" || strings.Contains(winPath, "%USERPROFILE%") {
		return ""
	}

	pathBin, err := exec.LookPath("wslpath")
	if err != nil {
		return ""
	}
	out, err = exec.CommandContext(ctx, pathBin, "-u", winPath).Output()
	if err != nil {
		return ""
	}
	unixPath := strings.TrimSpace(string(out))
	if !go_pkg_filesystem_reader.IsDir(unixPath) {
		return ""
	}
	return unixPath
}

var winSystemProfile = map[string]bool{
	"public":       true,
	"default":      true,
	"default user": true,
	"all users":    true,
	"defaultuser0": true,
}

func winHomeFromMount() string {
	for _, drive := range []string{"c", "d"} {
		dirs, err := go_pkg_filesystem_reader.ListDirs("/mnt/" + drive + "/Users")
		if err != nil {
			continue
		}
		var found string
		for _, one := range dirs {
			if winSystemProfile[strings.ToLower(one.Name)] {
				continue
			}
			if found != "" {
				return ""
			}
			found = one.Path
		}
		if found != "" {
			return found
		}
	}
	return ""
}

func hostNoteSection() string {
	info := getSystemInfo()
	if !info.wsl {
		return ""
	}
	winHome := info.home
	if winHome == "" {
		winHome = winHomeFallback
	}
	return strings.ReplaceAll(configs.WSLHost, "{{.WinHome}}", winHome)
}
