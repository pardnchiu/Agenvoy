package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registOpenFile() {
	toolregister.Regist(toolregister.Def{
		Name:        "open_file",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  false,
		Description: `Hands a file or URL to the OS default handler and returns the opened path: plays a video, shows an image, opens a PDF viewer, a system settings pane or a link.
Use for "open this file", "play the video", "show me the image", "open settings", and for open / xdg-open / start.
run_command cannot do this: its sandbox has no route to the app-launch service. Reading the contents instead → read_files.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute, ~ or work-directory-relative file path, or any scheme: URL passed through as is, e.g. '~/Downloads/report.pdf', 'relative/file.png', 'x-apple.systempreferences:com.apple.Desktop-Settings.extension', 'https://example.com'.",
				},
			},
			"required": []string{"path"},
		},
		Timeout: 10 * time.Second,
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			return OpenFile(ctx, e.WorkDir, e.SessionID, params.Path)
		},
	})
}

var urlSchemeRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]+:`)

func IsOpenURL(path string) bool {
	return urlSchemeRegex.MatchString(strings.TrimSpace(path))
}

func OpenFile(ctx context.Context, workDir, sessionID, path string) (string, error) {
	var absPath string
	if IsOpenURL(path) {
		absPath = strings.TrimSpace(path)
	} else {
		baseDir := workDir
		if baseDir == "" {
			baseDir = filesystem.DownloadDir
		}

		resolved, err := boundary.Resolve(sessionID, baseDir, path)
		if err != nil {
			return "", utilsmsg.Trace("boundary.Resolve", "", err)
		}
		if resolved == "" {
			return "", utilsmsg.New(utilsmsg.Invalid, "path", "required")
		}
		absPath = resolved
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", absPath)
	case "linux":
		bin, args, cmdErr := linuxOpenCmd(ctx, absPath)
		if cmdErr != nil {
			return "", utilsmsg.Trace("open_file", "", cmdErr)
		}
		cmd = exec.CommandContext(ctx, bin, args...)
	case "windows":
		cmd = exec.CommandContext(ctx, "explorer", absPath)
	default:
		return "", utilsmsg.New(utilsmsg.Unavailable, "open_file", "unsupported platform %s", runtime.GOOS)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%s\nError: %s", string(output), err.Error()), nil
	}

	return fmt.Sprintf("opened %s", absPath), nil
}

func linuxOpenCmd(ctx context.Context, target string) (string, []string, error) {
	if utils.IsWSL() {
		if winPath, err := wslToWindowsPath(ctx, target); err == nil {
			if bin, lookErr := exec.LookPath("cmd.exe"); lookErr == nil {
				return bin, []string{"/c", "start", "", winPath}, nil
			}
		}
	}
	if bin, err := exec.LookPath("wslview"); err == nil {
		return bin, []string{target}, nil
	}
	if bin, err := exec.LookPath("xdg-open"); err == nil {
		return bin, []string{target}, nil
	}
	return "", nil, utilsmsg.New(utilsmsg.Unavailable, "opener", "no opener available (cmd.exe/wslview/xdg-open not found)")
}

func wslToWindowsPath(ctx context.Context, path string) (string, error) {
	bin, err := exec.LookPath("wslpath")
	if err != nil {
		return "", err
	}
	out, err := exec.CommandContext(ctx, bin, "-w", path).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
