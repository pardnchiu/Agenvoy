package allowSkill

import (
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func read(path string) map[string]bool {
	out := make(map[string]bool)
	if !go_pkg_filesystem_reader.Exists(path) {
		return out
	}
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return out
	}
	for line := range strings.SplitSeq(text, "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		out[entry] = true
	}
	return out
}

func LoadGlobal() map[string]bool {
	return read(filesystem.AllowSkillGlobalPath())
}

func LoadProject(workDir string) map[string]bool {
	if strings.TrimSpace(workDir) == "" {
		return map[string]bool{}
	}
	return read(filesystem.AllowSkillProjectPath(workDir))
}

func LoadEffective(workDir string) map[string]bool {
	out := LoadGlobal()
	for name := range LoadProject(workDir) {
		out[name] = true
	}
	return out
}
