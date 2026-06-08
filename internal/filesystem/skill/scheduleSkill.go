package skill

import (
	"context"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func GetSchedule(name string) (string, error) {
	path := filesystem.ScheduleSkillPath(name)
	if !go_pkg_filesystem_reader.Exists(path) {
		return "", fmt.Errorf("schedule skill [%s] not found", name)
	}

	result, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem/reader ReadText [%s]: %w", path, err)
	}
	return strings.TrimSpace(skillBodyStripRegex.ReplaceAllString(result, "")), nil
}

func TrashSchedule(ctx context.Context, name string) error {
	dir := filesystem.ScheduleSkillDir(name)
	if !go_pkg_filesystem_reader.IsDir(dir) {
		return nil
	}

	if _, err := filesystem.TrashDir(dir, filesystem.ScheduleSkillTrashDir, name); err != nil {
		return err
	}

	filesystem.GitAutoCommit(ctx, filesystem.GitSkills, "trash", name)
	return nil
}
