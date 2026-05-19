package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

var (
	filesystemOnce  sync.Once
	AgenvoyDir      string
	ConfigPath      string
	UsagePath       string
	McpPath         string
	StoreDir        string
	SessionsDir     string
	ToolsDir       string
	APIToolsDir    string
	ScriptToolsDir string
	SystemToolsDir string
	ErrorsDir       string
	TasksPath       string
	CronsPath       string
	TelegramAuthPath string
	DiscordAuthPath  string
	SkillsDir            string
	SystemSkillsDir      string
	ScheduleSkillsDir    string
	ScheduleSkillTrashDir string
	DownloadDir          string

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
		UsagePath = filepath.Join(AgenvoyDir, "usage.json")
		McpPath = filepath.Join(AgenvoyDir, "mcp.json")

		StoreDir = filepath.Join(AgenvoyDir, ".store")
		SessionsDir = filepath.Join(AgenvoyDir, "sessions")
		ToolsDir = filepath.Join(AgenvoyDir, "tools")
		APIToolsDir = filepath.Join(ToolsDir, "api")
		ScriptToolsDir = filepath.Join(ToolsDir, "script")
		SystemToolsDir = filepath.Join(ToolsDir, ".system")
		ErrorsDir = filepath.Join(AgenvoyDir, "errors")
		TasksPath = filepath.Join(AgenvoyDir, "tasks.json")
		CronsPath = filepath.Join(AgenvoyDir, "crons.json")
		TelegramAuthPath = filepath.Join(AgenvoyDir, ".telegram")
		DiscordAuthPath = filepath.Join(AgenvoyDir, ".discord")

		SkillsDir = filepath.Join(AgenvoyDir, "skills")
		SystemSkillsDir = filepath.Join(SkillsDir, ".system")
		ScheduleSkillsDir = filepath.Join(SkillsDir, "scheduler")
		ScheduleSkillTrashDir = filepath.Join(ScheduleSkillsDir, ".Trash")

		LegacyAPIToolsDir = filepath.Join(AgenvoyDir, "api_tools")
		LegacyScriptToolsDir = filepath.Join(AgenvoyDir, "script_tools")

		systemDownloads := filepath.Join(homeDir, "Downloads")
		if go_pkg_filesystem_reader.IsDir(systemDownloads) {
			DownloadDir = systemDownloads
		} else {
			DownloadDir = filepath.Join(AgenvoyDir, "download")
		}

		WorkAgenvoyDir = filepath.Join(workDir, ".config", projectName)
		WorkAPIToolsDir = filepath.Join(WorkAgenvoyDir, "tools", "api")
		WorkScriptToolsDir = filepath.Join(WorkAgenvoyDir, "tools", "script")
		WorkSkillsDir = filepath.Join(WorkAgenvoyDir, "skills")

		LegacyWorkAPIToolsDir = filepath.Join(WorkAgenvoyDir, "api_tools")
		LegacyWorkScriptToolsDir = filepath.Join(WorkAgenvoyDir, "script_tools")
	})

	for _, dir := range []string{AgenvoyDir, DownloadDir} {
		if err = go_pkg_filesystem.CheckDir(dir, true); err != nil {
			return fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
		}
	}

	keychain.Init(projectName, AgenvoyDir)

	return nil
}

func HistoryPath(sessionID string) string {
	return filepath.Join(SessionsDir, sessionID, "history.md")
}

func InputHistoryPath(sessionID string) string {
	return filepath.Join(SessionsDir, sessionID, ".history")
}

func McpSessionPath(sessionID string) string {
	return filepath.Join(SessionsDir, sessionID, "mcp.json")
}

func PagePath(sessionID string) string {
	return filepath.Join(SessionsDir, sessionID, "page")
}

