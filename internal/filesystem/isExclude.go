package filesystem

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
)

type Exclude struct {
	File   string
	Negate bool
}

func IsExclude(workDir, absPath string) bool {
	excludes := listExcludes(workDir)
	excluded := false
	for _, exclude := range excludes {
		match, err := filepath.Match(exclude.File, filepath.Base(absPath))
		if err != nil {
			continue
		}

		if !match {
			match = strings.Contains(absPath, "/"+exclude.File+"/") ||
				strings.HasPrefix(absPath, exclude.File+"/")
		}
		if match {
			excluded = !exclude.Negate
		}
	}
	return excluded
}

func listExcludes(dir string) []Exclude {
	var defaults []string
	if err := json.Unmarshal(configs.ExcludeList, &defaults); err != nil {
		slog.Warn("json.Unmarshal",
			slog.String("error", err.Error()))
	}

	newFiles := make([]Exclude, 0, len(defaults))
	for _, line := range defaults {
		if ef, ok := checkLine(line); ok {
			newFiles = append(newFiles, ef)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return newFiles
	}

	for _, entry := range entries {
		// * to fit file name like .*ignore
		name := entry.Name()
		if entry.IsDir() ||
			!strings.HasSuffix(name, "ignore") ||
			!strings.HasPrefix(name, ".") {
			continue
		}

		newFiles = append(newFiles, parseIgnore(filepath.Join(dir, name))...)
	}

	return newFiles
}

func parseIgnore(path string) []Exclude {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var files []Exclude
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if ef, ok := checkLine(scanner.Text()); ok {
			files = append(files, ef)
		}
	}

	return files
}

func checkLine(raw string) (Exclude, bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return Exclude{}, false
	}

	negate := false
	if strings.HasPrefix(line, "!") {
		negate = true
		line = strings.TrimPrefix(line, "!")
	}

	line = strings.TrimPrefix(line, "/")
	line = strings.TrimSuffix(line, "/")
	if line == "" {
		return Exclude{}, false
	}

	return Exclude{
		File:   line,
		Negate: negate,
	}, true
}
