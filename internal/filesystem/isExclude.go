package filesystem

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
)

type exclude struct {
	file   string
	negate bool
}

var (
	invailedRegex = regexp.MustCompile(`^!{2,}`)
)

// * not ban, just skipped the folder that package manager install
func isExclude(workDir, absPath string) bool {
	relPath, err := filepath.Rel(workDir, absPath)
	if err != nil {
		relPath = absPath
	}

	isExcluded := false
	for _, exclude := range excludes(workDir) {
		match, err := filepath.Match(exclude.file, filepath.Base(relPath))
		if err != nil {
			continue
		}

		if !match {
			match = strings.HasPrefix(relPath, exclude.file+"/") ||
				strings.Contains(relPath, "/"+exclude.file+"/")
		}
		if match {
			isExcluded = !exclude.negate
		}
	}
	return isExcluded
}

func excludes(dir string) []exclude {
	seen := make(map[exclude]struct{})
	var newExcludes []exclude

	add := func(ef exclude) {
		if _, ok := seen[ef]; ok {
			return
		}
		seen[ef] = struct{}{}
		newExcludes = append(newExcludes, ef)
	}

	var raw []string
	if err := json.Unmarshal(configs.ExcludeList, &raw); err != nil {
		slog.Warn("json.Unmarshal",
			slog.String("error", err.Error()))
	}
	for _, path := range raw {
		if ef, ok := checkFormat(path); ok {
			add(ef)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return newExcludes
	}

	for _, entry := range entries {
		// * to fit file name like .*ignore
		name := entry.Name()
		if entry.IsDir() ||
			!strings.HasSuffix(name, "ignore") ||
			!strings.HasPrefix(name, ".") {
			continue
		}

		for _, ef := range parseIgnore(filepath.Join(dir, name)) {
			add(ef)
		}
	}

	return newExcludes
}

func checkFormat(raw string) (exclude, bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return exclude{}, false
	}

	if invailedRegex.MatchString(line) {
		return exclude{}, false
	}

	negate := false
	if strings.HasPrefix(line, "!") {
		negate = true

		line = strings.TrimPrefix(line, "!")
		if line == "" {
			return exclude{}, false
		}
	}

	line = strings.TrimPrefix(line, "/")
	line = strings.TrimSuffix(line, "/")
	if line == "" {
		return exclude{}, false
	}

	return exclude{
		file:   line,
		negate: negate,
	}, true
}

func parseIgnore(path string) []exclude {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var files []exclude
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if ef, ok := checkFormat(scanner.Text()); ok {
			files = append(files, ef)
		}
	}

	return files
}
