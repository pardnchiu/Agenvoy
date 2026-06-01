package skill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	if err := go_pkg_filesystem.CheckDir(filesystem.ScheduleSkillTrashDir, true); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir [%s]: %w", dir, err)
	}

	dst := filepath.Join(filesystem.ScheduleSkillTrashDir, name)
	if go_pkg_filesystem_reader.Exists(dst) {
		dst = filepath.Join(filesystem.ScheduleSkillTrashDir, fmt.Sprintf("%s-%d", name, time.Now().Unix()))
	}
	if err := os.Rename(dir, dst); err != nil {
		return err
	}

	AutoCommit(ctx, "trash", name)

	return nil
}
