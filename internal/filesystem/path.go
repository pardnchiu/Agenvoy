package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

var (
	filesystemOnce          sync.Once
	AgenvoyDir              string
	ConfigPath              string
	DaemonLogPath           string
	UsagePath               string
	McpPath                 string
	StoreDir                string
	HistoryDBPath           string
	SessionsDir             string
	ToolsDir                string
	APIToolsDir             string
	ScriptToolsDir          string
	SystemToolsDir          string
	ExtensionAPIToolsDir    string
	ExtensionScriptToolsDir string
	ToolGitignorePath       string
	ScriptToolTrashDir      string
	ErrorsDir               string
	TasksPath               string
	CronsPath               string
	TelegramAuthPath        string
	DiscordAuthPath         string
	SkillsDir               string
	SkillGitDir             string
	SkillGitignorePath      string
	SystemSkillsDir         string
	ScheduleSkillsDir       string
	ScheduleSkillTrashDir   string
	SkillTrashDir           string
	DownloadDir             string
	DownloadTrashDir        string
	SessionsTrashDir        string
	AllowSkillGlobalPath    string
	PromptsDir              string
	KuradbDir               string
	KuradbEndpointPath      string

	WorkAgenvoyDir     string
	WorkAPIToolsDir    string
	WorkScriptToolsDir string
	WorkSkillsDir      string

	// will deprecated
	LegacyAPIToolsDir        string
	LegacyScriptToolsDir     string
	LegacyWorkAPIToolsDir    string
	LegacyWorkScriptToolsDir string
)

const (
	projectName = "agenvoy"
)

func Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("os.UserHomeDir: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("os.Getwd: %w", err)
	}

	filesystemOnce.Do(func() {
		AgenvoyDir = filepath.Join(homeDir, ".config", projectName)
		ConfigPath = filepath.Join(AgenvoyDir, "config.json")
		DaemonLogPath = filepath.Join(AgenvoyDir, "daemon.log")
		UsagePath = filepath.Join(AgenvoyDir, "usage.json")
		McpPath = filepath.Join(AgenvoyDir, "mcp.json")

		StoreDir = filepath.Join(AgenvoyDir, ".store")
		HistoryDBPath = filepath.Join(StoreDir, "history.db")
		SessionsDir = filepath.Join(AgenvoyDir, "sessions")
		ToolsDir = filepath.Join(AgenvoyDir, "tools")
		APIToolsDir = filepath.Join(ToolsDir, "api")
		ScriptToolsDir = filepath.Join(ToolsDir, "script")
		SystemToolsDir = filepath.Join(ToolsDir, ".system")
		ExtensionAPIToolsDir = filepath.Join(ToolsDir, ".extension", "api")
		ExtensionScriptToolsDir = filepath.Join(ToolsDir, ".extension", "script")
		ToolGitignorePath = filepath.Join(ToolsDir, ".gitignore")
		ScriptToolTrashDir = filepath.Join(ScriptToolsDir, ".Trash")
		ErrorsDir = filepath.Join(AgenvoyDir, "errors")
		TasksPath = filepath.Join(AgenvoyDir, "tasks.json")
		CronsPath = filepath.Join(AgenvoyDir, "crons.json")
		TelegramAuthPath = filepath.Join(AgenvoyDir, ".telegram")
		DiscordAuthPath = filepath.Join(AgenvoyDir, ".discord")

		SkillsDir = filepath.Join(AgenvoyDir, "skills")
		SkillGitDir = filepath.Join(SkillsDir, ".git")
		SkillGitignorePath = filepath.Join(SkillsDir, ".gitignore")
		SystemSkillsDir = filepath.Join(SkillsDir, ".system")
		ScheduleSkillsDir = filepath.Join(SkillsDir, "scheduler")
		ScheduleSkillTrashDir = filepath.Join(ScheduleSkillsDir, ".Trash")
		SkillTrashDir = filepath.Join(SkillsDir, ".Trash")

		LegacyAPIToolsDir = filepath.Join(AgenvoyDir, "api_tools")
		LegacyScriptToolsDir = filepath.Join(AgenvoyDir, "script_tools")

		DownloadDir = filepath.Join(AgenvoyDir, "download")
		DownloadTrashDir = filepath.Join(DownloadDir, ".Trash")
		SessionsTrashDir = filepath.Join(SessionsDir, ".Trash")
		AllowSkillGlobalPath = filepath.Join(AgenvoyDir, "allow_skill")
		PromptsDir = filepath.Join(AgenvoyDir, "prompts")

		KuradbDir = filepath.Join(homeDir, ".config", "kuradb")
		KuradbEndpointPath = filepath.Join(KuradbDir, "endpoint")

		WorkAgenvoyDir = filepath.Join(workDir, ".config", projectName)
		WorkAPIToolsDir = filepath.Join(WorkAgenvoyDir, "tools", "api")
		WorkScriptToolsDir = filepath.Join(WorkAgenvoyDir, "tools", "script")
		WorkSkillsDir = filepath.Join(WorkAgenvoyDir, "skills")

		LegacyWorkAPIToolsDir = filepath.Join(WorkAgenvoyDir, "api_tools")
		LegacyWorkScriptToolsDir = filepath.Join(WorkAgenvoyDir, "script_tools")
	})

	for _, dir := range []string{
		AgenvoyDir,
		DownloadDir,
		DownloadTrashDir,
		SessionsTrashDir,
		ExtensionAPIToolsDir,
		ExtensionScriptToolsDir,
	} {
		if err = go_pkg_filesystem.CheckDir(dir, true); err != nil {
			return fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
		}
	}

	keychain.Init(projectName, AgenvoyDir)

	return nil
}

func SessionDir(sessionID string) string {
	return filepath.Join(SessionsDir, sessionID)
}

func SessionConfigPath(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "config.json")
}

func StatusPath(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "status.json")
}

func BotPath(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "bot.md")
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

func SummaryMetaPath(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "summary.meta.json")
}

func InputHistoryPath(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), ".history")
}

func PendingDir(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "pending")
}

func PendingMetaPath(sessionID, taskHash string) string {
	return filepath.Join(PendingDir(sessionID), taskHash+".json")
}

func TaskHistoryDir(sessionID string) string {
	return filepath.Join(SessionDir(sessionID), "history")
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

func GetKuradbEndpoint() (string, error) {
	path := KuradbEndpointPath
	if !go_pkg_filesystem_reader.Exists(path) {
		return "", fmt.Errorf("endpoint file not found: %s", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("os.ReadFile %s: %w", path, err)
	}
	url := strings.TrimSpace(string(raw))
	if url == "" {
		return "", fmt.Errorf("endpoint file %s is empty", path)
	}
	return url, nil
}

func TrashDir(src, trashBase, name string) (string, error) {
	if err := go_pkg_filesystem.CheckDir(trashBase, true); err != nil {
		return "", fmt.Errorf("go_pkg_filesystem.CheckDir [%s]: %w", trashBase, err)
	}
	dst := filepath.Join(trashBase, name)
	if go_pkg_filesystem_reader.Exists(dst) {
		dst = filepath.Join(trashBase, fmt.Sprintf("%s-%d", name, time.Now().Unix()))
	}
	if err := os.Rename(src, dst); err != nil {
		return "", fmt.Errorf("os.Rename [%s → %s]: %w", src, dst, err)
	}
	return dst, nil
}
