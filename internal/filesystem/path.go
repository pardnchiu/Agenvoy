package filesystem

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var (
	filesystemOnce          sync.Once
	AgenvoyDir              string
	ConfigPath              string
	DaemonLogPath           string
	DaemonSocketPath        string
	McpPath                 string
	StoreDir                string
	HistoryDBPath           string
	StoreTempDir            string
	SessionsDir             string
	ToolsDir                string
	ScriptToolsDir          string
	SystemToolsDir          string
	ExtensionScriptToolsDir string
	ScriptToolTrashDir      string
	ErrorsDir               string
	TasksPath               string
	CronsPath               string
	TelegramAuthPath        string
	DiscordAuthPath         string
	LineAuthPath            string
	SkillsDir               string
	SystemSkillsDir         string
	SystemDesignDir         string
	ScheduleSkillsDir       string
	ScheduleSkillTrashDir   string
	SkillTrashDir           string
	DownloadDir             string
	DownloadTrashDir        string
	SessionsTrashDir        string
	AllowSkillGlobalPath    string
	AllowToolGlobalPath     string
	PromptsDir              string
	VendorDir               string

	WorkAgenvoyDir     string
	WorkScriptToolsDir string
	WorkSkillsDir      string
)

const (
	projectName      = "agenvoy"
	TrashStampLayout = "20060102_150405.000"
)

func Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os.UserHomeDir", "", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os.Getwd", "", err)
	}

	filesystemOnce.Do(func() {
		AgenvoyDir = filepath.Join(homeDir, ".config", projectName)
		ConfigPath = filepath.Join(AgenvoyDir, "config.json")
		DaemonLogPath = filepath.Join(AgenvoyDir, "daemon.log")
		DaemonSocketPath = filepath.Join(AgenvoyDir, "daemon.sock")
		McpPath = filepath.Join(AgenvoyDir, "mcp.json")

		StoreDir = filepath.Join(AgenvoyDir, ".store")
		HistoryDBPath = filepath.Join(StoreDir, "history.db")
		StoreTempDir = filepath.Join(StoreDir, "temp")
		SessionsDir = filepath.Join(AgenvoyDir, "sessions")
		ToolsDir = filepath.Join(AgenvoyDir, "tools")
		ScriptToolsDir = filepath.Join(ToolsDir, "script")
		SystemToolsDir = filepath.Join(ToolsDir, ".system")
		ExtensionScriptToolsDir = filepath.Join(ToolsDir, ".extension", "script")
		ScriptToolTrashDir = filepath.Join(ScriptToolsDir, ".Trash")
		ErrorsDir = filepath.Join(AgenvoyDir, "errors")
		TasksPath = filepath.Join(AgenvoyDir, "tasks.json")
		CronsPath = filepath.Join(AgenvoyDir, "crons.json")
		TelegramAuthPath = filepath.Join(AgenvoyDir, ".telegram")
		DiscordAuthPath = filepath.Join(AgenvoyDir, ".discord")
		LineAuthPath = filepath.Join(AgenvoyDir, ".line")

		SkillsDir = filepath.Join(AgenvoyDir, "skills")
		SystemSkillsDir = filepath.Join(SkillsDir, ".system")
		SystemDesignDir = filepath.Join(SkillsDir, ".system_design")
		ScheduleSkillsDir = filepath.Join(SkillsDir, "scheduler")
		ScheduleSkillTrashDir = filepath.Join(ScheduleSkillsDir, ".Trash")
		SkillTrashDir = filepath.Join(SkillsDir, ".Trash")

		DownloadDir = filepath.Join(AgenvoyDir, "download")
		DownloadTrashDir = filepath.Join(DownloadDir, ".Trash")
		SessionsTrashDir = filepath.Join(SessionsDir, ".Trash")
		AllowSkillGlobalPath = filepath.Join(AgenvoyDir, "allow_skill")
		AllowToolGlobalPath = filepath.Join(AgenvoyDir, "allow_tool")
		PromptsDir = filepath.Join(AgenvoyDir, "prompts")
		VendorDir = filepath.Join(AgenvoyDir, "vendor")

		WorkAgenvoyDir = filepath.Join(workDir, ".config", projectName)
		WorkScriptToolsDir = filepath.Join(WorkAgenvoyDir, "tools", "script")
		WorkSkillsDir = filepath.Join(WorkAgenvoyDir, "skills")
	})

	for _, dir := range []string{
		AgenvoyDir,
		DownloadDir,
		DownloadTrashDir,
		SessionsTrashDir,
		ExtensionScriptToolsDir,
	} {
		if err = go_pkg_filesystem.CheckDir(dir, true); err != nil {
			return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", dir, err)
		}
	}

	keychain.Init(projectName, AgenvoyDir)

	return nil
}

func SessionDir(sessionID string) string {
	return filepath.Join(SessionsDir, sessionID)
}

func ActionLogPath(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "action.log")
}

func HistoryPath(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "history.json")
}

func SummaryPath(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "summary.json")
}

func SummaryCursorPath(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), ".summary_cursor")
}

func InputHistoryPath(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), ".cmd_history")
}

func BackgroundDir(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "background")
}

func BackgroundLogPath(sessionID, id string) string {
	return filepath.Join(BackgroundDir(sessionID), id+".log")
}

func PendingDir(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "pending")
}

func PendingMetaPath(sessionID, taskHash string) string {
	return filepath.Join(PendingDir(sessionID), taskHash+".json")
}

func AllowSkillProjectPath(workDir string) string {
	return filepath.Join(workDir, "."+projectName, "allow_skill")
}

func AllowToolPath(workDir string) string {
	return filepath.Join(workDir, "."+projectName, "allow_list")
}

func ScheduleSkillDir(name string) string {
	return filepath.Join(ScheduleSkillsDir, name)
}

func ScheduleSkillPath(name string) string {
	return filepath.Join(ScheduleSkillDir(name), "SKILL.md")
}

func CopyToStoreTemp(src string) (string, error) {
	dst, err := storeTempPath(src)
	if err != nil {
		return "", err
	}
	if err := CopyPath(src, dst); err != nil {
		return "", utilsmsg.Trace("CopyPath", src+" → "+dst, err)
	}
	return dst, nil
}

func MoveToStoreTemp(src string) (string, error) {
	dst, err := storeTempPath(src)
	if err != nil {
		return "", err
	}
	if err := go_pkg_filesystem.Move(src, dst); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.Move", src+" → "+dst, err)
	}
	return dst, nil
}

func storeTempPath(src string) (string, error) {
	if StoreTempDir == "" {
		return "", utilsmsg.New(utilsmsg.Internal, "filesystem.Init", "has not run: no trash directory to move %s into", src)
	}

	mirrored := filepath.Join(StoreTempDir, strings.TrimPrefix(src, string(filepath.Separator)))
	ext := filepath.Ext(mirrored)
	base := strings.TrimSuffix(mirrored, ext)

	stamp := time.Now().Format(TrashStampLayout)
	dst := fmt.Sprintf("%s_%s%s", base, stamp, ext)
	if !go_pkg_filesystem_reader.Exists(dst) {
		return dst, nil
	}
	return fmt.Sprintf("%s_%s_%d%s", base, stamp, time.Now().UnixNano(), ext), nil
}

func CopyPath(src, dst string) error {
	if !go_pkg_filesystem_reader.IsDir(src) {
		return go_pkg_filesystem.Copy(src, dst)
	}

	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return utilsmsg.Wrap(utilsmsg.Internal, "filepath.Rel", path, err)
		}
		target := filepath.Join(dst, rel)

		if entry.IsDir() {
			if err := go_pkg_filesystem.CheckDir(target, true); err != nil {
				return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", target, err)
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if err := go_pkg_filesystem.Copy(path, target); err != nil {
			return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.Copy", path+" → "+target, err)
		}
		return nil
	})
}

func ErrorDir(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "tool_errors")
}

func ErrorPath(sessionID, hash string) string {
	return filepath.Join(ErrorDir(sessionID), hash+".json")
}

func TrashDir(src, trashBase, name string) (string, error) {
	if err := go_pkg_filesystem.CheckDir(trashBase, true); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", trashBase, err)
	}
	dst := filepath.Join(trashBase, name)
	if go_pkg_filesystem_reader.Exists(dst) {
		dst = filepath.Join(trashBase, fmt.Sprintf("%s-%d", name, time.Now().Unix()))
	}
	if err := os.Rename(src, dst); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os.Rename", src+" → "+dst, err)
	}
	return dst, nil
}

func OutputDir() string {
	if raw := ConfigOutputDir; raw != "" {
		dir, err := ResolveOutputDir(raw)
		if err == nil {
			return dir
		}
		slog.Warn("output_dir unusable, falling back to the default",
			slog.String("dir", raw),
			slog.String("error", err.Error()))
	}
	return defaultOutputDir()
}

func defaultOutputDir() string {
	if home, err := os.UserHomeDir(); err == nil && go_pkg_filesystem_reader.IsDir(filepath.Join(home, "Downloads")) {
		return filepath.Join(home, "Downloads")
	}
	return DownloadDir
}

func ResolveOutputDir(raw string) (string, error) {
	dir := strings.TrimSpace(raw)
	if dir == "" {
		return defaultOutputDir(), nil
	}
	if rest, ok := strings.CutPrefix(dir, "~"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Internal, "os.UserHomeDir", "", err)
		}
		dir = filepath.Join(home, rest)
	}
	dir = filepath.Clean(dir)
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", dir, err)
	}
	return dir, nil
}
