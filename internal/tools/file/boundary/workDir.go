package boundary

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func OutsideWorkDir(sessionID, workDir, toolName, toolArgs string) []string {
	home, err := filesystem.RealHome()
	if err != nil {
		return nil
	}
	dir, err := resolveRaw("", baseDir(workDir))
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var out []string
	for _, one := range targetPaths(toolName, toolArgs) {
		abs, err := resolveRaw(dir, one)
		if err != nil || seen[abs] {
			continue
		}
		seen[abs] = true
		if filesystem.IsWithin(dir, abs) || !filesystem.IsWithin(home, abs) || filesystem.IsSensitivePath(abs) || filesystem.IsDeniedPath(abs) || isGranted(sessionID, abs) {
			continue
		}
		out = append(out, abs)
	}
	return out
}

func targetPaths(toolName, toolArgs string) []string {
	switch toolName {
	case "edit_file":
		return rawPaths(toolName, toolArgs)
	case "run_command", "background_command":
		var p struct {
			Argv       []string `json:"argv"`
			WritePaths []string `json:"write_paths"`
		}
		if json.Unmarshal([]byte(toolArgs), &p) != nil {
			return nil
		}
		list := append(append([]string{}, p.WritePaths...), RemoveTargets(p.Argv)...)
		for _, word := range argvWords(p.Argv) {
			if path, ok := pathToken(word); ok {
				list = append(list, path)
			}
		}
		return list
	}
	return nil
}

func argvWords(argv []string) []string {
	if len(argv) < 3 || argv[1] != "-c" {
		return argv
	}
	switch filepath.Base(argv[0]) {
	case "sh", "bash", "zsh":
	default:
		return argv
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(argv[2]), "")
	if err != nil {
		return argv
	}
	var words []string
	syntax.Walk(file, func(node syntax.Node) bool {
		switch v := node.(type) {
		case *syntax.Lit:
			words = append(words, v.Value)
		case *syntax.SglQuoted:
			words = append(words, v.Value)
		}
		return true
	})
	return words
}

func pathToken(word string) (string, bool) {
	word = strings.TrimSpace(word)
	if _, value, ok := strings.Cut(word, "="); ok && strings.HasPrefix(word, "-") {
		word = value
	}
	if word == "" || strings.Contains(word, "://") {
		return "", false
	}
	if strings.HasPrefix(word, "/") || word == "~" || strings.HasPrefix(word, "~/") || word == ".." || strings.HasPrefix(word, "../") || strings.Contains(word, "/../") {
		return word, true
	}
	return "", false
}
