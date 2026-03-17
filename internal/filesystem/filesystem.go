package filesystem

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	once         sync.Once
	AgenvoyDir   string
	ConfigPath   string
	SessionsDir  string
	APIsDir      string
	ErrorsDir    string
	SchedulerDir string
	TasksPath    string
	CronsPath    string
	ScriptsDir   string
	SkillsDir    string
	ToolsDir     string

	WorkAgenvoyDir string
	WorkAPIsDir    string
	WorkSkillsDir  string
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

	once.Do(func() {
		AgenvoyDir = filepath.Join(homeDir, ".config", projectName)
		ConfigPath = filepath.Join(AgenvoyDir, "config.json")

		SessionsDir = filepath.Join(AgenvoyDir, "sessions")
		APIsDir = filepath.Join(AgenvoyDir, "apis")
		ErrorsDir = filepath.Join(AgenvoyDir, "errors")
		SchedulerDir = filepath.Join(AgenvoyDir, "scheduler")
		TasksPath = filepath.Join(SchedulerDir, "tasks.json")
		CronsPath = filepath.Join(SchedulerDir, "crons.json")
		ScriptsDir = filepath.Join(SchedulerDir, "scripts")

		SkillsDir = filepath.Join(AgenvoyDir, "skills")
		ToolsDir = filepath.Join(AgenvoyDir, "tools")

		WorkAgenvoyDir = filepath.Join(workDir, ".config", projectName)
		WorkAPIsDir = filepath.Join(WorkAgenvoyDir, "apis")
		WorkSkillsDir = filepath.Join(WorkAgenvoyDir, "skills")
	})

	if err = os.MkdirAll(AgenvoyDir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}

	return nil
}

func ReadFile(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("os.ReadFile: %w", err)
	}

	return string(bytes), nil
}

func ReadFileSlice(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("os.Open: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func WriteFile(path, content string, permission os.FileMode) error {
	absDir := filepath.Dir(path)
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}
	// * ensure atomic write:
	// * pre-save data as temp
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), permission); err != nil {
		return fmt.Errorf("os.WriteFile: %w", err)
	}
	// * rename temp to target
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		slog.Warn("os.Rename",
			slog.String("tmp", tmp),
			slog.String("error", err.Error()))
		return fmt.Errorf("os.Rename: %w", err)
	}
	return nil
}

func WriteFileWithLines(path string, lines []string, permission os.FileMode) error {
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}
	return WriteFile(path, content, permission)
}

func GetAbsPath(dir, path string) (string, error) {
	// * format the path to abs path
	var resolved string
	if !filepath.IsAbs(path) {
		resolved = filepath.Join(dir, path)
	} else {
		resolved = filepath.Clean(path)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(resolved, filepath.Clean(homeDir)+string(filepath.Separator)) {
		return "", fmt.Errorf("only allow user home: %s", path)
	}

	if isDenied(resolved) {
		return "", fmt.Errorf("access denied: %s", path)
	}

	return resolved, nil
}
