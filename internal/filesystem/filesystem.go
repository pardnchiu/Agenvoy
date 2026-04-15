package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/pardnchiu/go-utils/filesystem/keychain"
)

var (
	filesystemOnce      sync.Once
	AgenvoyDir          string
	ConfigPath          string
	UsagePath           string
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
		if info, statErr := os.Stat(systemDownloads); statErr == nil && info.IsDir() {
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
		if err = os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("os.MkdirAll: %w", err)
		}
	}

	keychain.Init(projectName, AgenvoyDir)

	return nil
}

func IsMatch(patterns, parts []string) bool {
	if len(patterns) == 0 {
		return len(parts) == 0
	}

	pattern := patterns[0]
	if pattern == "**" {
		rest := patterns[1:]
		for len(rest) > 0 && rest[0] == "**" {
			rest = rest[1:]
		}
		if len(rest) == 0 {
			return true
		}
		for i := 0; i <= len(parts); i++ {
			if IsMatch(rest, parts[i:]) {
				return true
			}
		}
		return false
	}

	if len(parts) == 0 {
		return false
	}

	match, err := filepath.Match(pattern, parts[0])
	if err != nil || !match {
		return false
	}
	return IsMatch(patterns[1:], parts[1:])
}

func HistoryPath(sessionID string) string {
	return filepath.Join(SessionsDir, sessionID, "history.md")
}
