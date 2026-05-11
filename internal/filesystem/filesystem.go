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
	filesystemOnce      sync.Once
	AgenvoyDir          string
	ConfigPath          string
	UsagePath           string
	McpPath             string
	StoreDir            string
	SessionsDir         string
	APIToolsDir         string
	ScriptToolsDir      string
	ErrorsDir           string
	SchedulerDir        string
	SchedulerStateDir   string
	SchedulerRecordsDir string
	TasksPath           string
	CronsPath           string
	ScriptsDir          string
	SkillsDir           string
	SystemSkillsDir     string
	ToolsDir            string
	DownloadDir         string

	WorkAgenvoyDir     string
	WorkAPIToolsDir    string
	WorkScriptToolsDir string
	WorkSkillsDir      string

	ToolFetchPage      string
	ToolSearchWeb      string
	ToolFetchGoogleRSS string
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
		APIToolsDir = filepath.Join(AgenvoyDir, "api_tools")
		ScriptToolsDir = filepath.Join(AgenvoyDir, "script_tools")
		ErrorsDir = filepath.Join(AgenvoyDir, "errors")
		SchedulerDir = filepath.Join(AgenvoyDir, "scheduler")
		SchedulerStateDir = filepath.Join(SchedulerDir, "state")
		SchedulerRecordsDir = filepath.Join(SchedulerDir, "cron_records")
		TasksPath = filepath.Join(SchedulerDir, "tasks.json")
		CronsPath = filepath.Join(SchedulerDir, "crons.json")
		ScriptsDir = filepath.Join(SchedulerDir, "scripts")

		SkillsDir = filepath.Join(AgenvoyDir, "skills")
		SystemSkillsDir = filepath.Join(SkillsDir, ".system")
		ToolsDir = filepath.Join(AgenvoyDir, "tools")
		ToolFetchPage = filepath.Join(ToolsDir, "fetch_page")
		ToolSearchWeb = filepath.Join(ToolsDir, "search_web")
		ToolFetchGoogleRSS = filepath.Join(ToolsDir, "google_rss")

		systemDownloads := filepath.Join(homeDir, "Downloads")
		if go_pkg_filesystem_reader.IsDir(systemDownloads) {
			DownloadDir = systemDownloads
		} else {
			DownloadDir = filepath.Join(AgenvoyDir, "download")
		}

		WorkAgenvoyDir = filepath.Join(workDir, ".config", projectName)
		WorkAPIToolsDir = filepath.Join(WorkAgenvoyDir, "api_tools")
		WorkScriptToolsDir = filepath.Join(WorkAgenvoyDir, "script_tools")
		WorkSkillsDir = filepath.Join(WorkAgenvoyDir, "skills")
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

const AllowListRelPath = ".agenvoy/allow_list"

func AllowListPath(workDir string) string {
	return filepath.Join(workDir, AllowListRelPath)
}

func AllowListDir(workDir string) string {
	return filepath.Join(workDir, filepath.Dir(AllowListRelPath))
}
