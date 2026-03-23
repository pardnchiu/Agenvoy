package filesystem

import (
	"bufio"
	"errors"
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
	UsagePath    string
	SessionsDir  string
	APIsDir      string
	ErrorsDir    string
	SchedulerDir string
	TasksPath    string
	CronsPath    string
	ScriptsDir   string
	SkillsDir    string
	ToolsDir     string
	DownloadDir  string

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
		UsagePath = filepath.Join(AgenvoyDir, "usage.json")

		SessionsDir = filepath.Join(AgenvoyDir, "sessions")
		APIsDir = filepath.Join(AgenvoyDir, "apis")
		ErrorsDir = filepath.Join(AgenvoyDir, "errors")
		SchedulerDir = filepath.Join(AgenvoyDir, "scheduler")
		TasksPath = filepath.Join(SchedulerDir, "tasks.json")
		CronsPath = filepath.Join(SchedulerDir, "crons.json")
		ScriptsDir = filepath.Join(SchedulerDir, "scripts")

		SkillsDir   = filepath.Join(AgenvoyDir, "skills")
		ToolsDir    = filepath.Join(AgenvoyDir, "tools")
		DownloadDir = filepath.Join(AgenvoyDir, "download")

		WorkAgenvoyDir = filepath.Join(workDir, ".config", projectName)
		WorkAPIsDir = filepath.Join(WorkAgenvoyDir, "apis")
		WorkSkillsDir = filepath.Join(WorkAgenvoyDir, "skills")
	})

	for _, dir := range []string{AgenvoyDir, DownloadDir} {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("os.MkdirAll: %w", err)
		}
	}

	return nil
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
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(dir, path)
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if errors.Is(err, os.ErrNotExist) {
		realParent, parentErr := filepath.EvalSymlinks(filepath.Dir(absPath))
		if parentErr != nil {
			return "", fmt.Errorf("filepath.EvalSymlinks: %w", parentErr)
		}
		realPath = filepath.Join(realParent, filepath.Base(absPath))
		err = nil
	}
	if err != nil {
		return "", fmt.Errorf("filepath.EvalSymlinks: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(realPath, filepath.Clean(homeDir)+string(filepath.Separator)) {
		return "", fmt.Errorf("only allow user home: %s", path)
	}

	if isDenied(realPath) {
		return "", fmt.Errorf("access denied: %s", path)
	}

	return realPath, nil
}

func WalkFiles(dirs ...string) ([]string, error) {
	if len(dirs) == 0 || len(dirs) > 2 {
		return nil, fmt.Errorf("invalid dir: %d", len(dirs))
	}
	workDir := dirs[0]
	subDir := workDir
	if len(dirs) == 2 {
		subDir = dirs[1]
	}

	absPath, err := GetAbsPath(workDir, subDir)
	if err != nil {
		return nil, fmt.Errorf("GetAbsPath: %w", err)
	}

	var files []string
	err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("filepath.WalkDir",
				slog.String("error", err.Error()))
			return nil
		}

		if IsExclude(workDir, path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(absPath, path)
		if err != nil {
			slog.Warn("filepath.Rel",
				slog.String("error", err.Error()))
			return nil
		}
		if rel == "." {
			return nil
		}

		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf(" filepath.WalkDir: %w", err)
	}
	return files, nil
}

func ListDir(dirs ...string) ([]string, error) {
	if len(dirs) == 0 || len(dirs) > 2 {
		return nil, fmt.Errorf("invalid dir: %d", len(dirs))
	}
	workDir := dirs[0]
	subDir := workDir
	if len(dirs) == 2 {
		subDir = dirs[1]
	}

	absPath, err := GetAbsPath(workDir, subDir)
	if err != nil {
		return nil, fmt.Errorf("GetAbsPath: %w", err)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("os.ReadDir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		newPath := filepath.Join(absPath, entry.Name())
		if IsExclude(workDir, newPath) {
			continue
		}

		if entry.IsDir() {
			files = append(files, entry.Name()+"/")
		} else {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func IsMatch(patterns, parts []string) bool {
	if len(patterns) == 0 {
		return len(parts) == 0
	}

	pattern := patterns[0]
	if pattern == "**" {
		rest := patterns[1:]
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
