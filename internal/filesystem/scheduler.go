package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

var (
	skillRegex = regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)
)

func ScheduleSkillDir(name string) string {
	return filepath.Join(ScheduleSkillsDir, name)
}

func ScheduleSkillPath(name string) string {
	return filepath.Join(ScheduleSkillDir(name), "SKILL.md")
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

func TrashScheduleSkill(name string) error {
	src := ScheduleSkillDir(name)
	if !go_pkg_filesystem_reader.IsDir(src) {
		return nil
	}
	if err := go_pkg_filesystem.CheckDir(ScheduleSkillTrashDir, true); err != nil {
		return fmt.Errorf("CheckDir trash: %w", err)
	}
	dst := filepath.Join(ScheduleSkillTrashDir, name)
	if go_pkg_filesystem_reader.Exists(dst) {
		dst = filepath.Join(ScheduleSkillTrashDir, fmt.Sprintf("%s-%d", name, time.Now().Unix()))
	}
	return os.Rename(src, dst)
}
