package filesystem

import (
	"path/filepath"
	"regexp"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

var (
	skillRegex = regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)
)

func ScheduleSkillPath(name string) string {
	return filepath.Join(SkillsDir, "scheduler", name, "SKILL.md")
}

func ScheduleSkillExists(name string) bool {
	return go_pkg_filesystem_reader.Exists(ScheduleSkillPath(name))
}

func ScheduleSkillBody(name string) (string, error) {
	content, err := go_pkg_filesystem.ReadText(ScheduleSkillPath(name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(skillRegex.ReplaceAllString(content, "")), nil
}
