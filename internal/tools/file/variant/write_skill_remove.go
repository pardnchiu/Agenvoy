package variant

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func writeSkillRemove(ctx context.Context, e *tooltypes.Executor, name string) (string, error) {
	name = strings.TrimSpace(name)
	if err := filesystem.CheckDirName("name", name); err != nil {
		return "", err
	}

	dir := filepath.Join(filesystem.SkillsDir, name)
	if !go_pkg_filesystem_reader.IsDir(dir) {
		return "", utilsmsg.New(utilsmsg.NotFound, "skill", "%q does not exist", name)
	}

	trashPath, err := filesystem.TrashDir(dir, filesystem.SkillTrashDir, name)
	if err != nil {
		return "", err
	}
	recordRemoval(ctx, e, dir, trashPath, "write_skill")

	return fmt.Sprintf("trashed: %s", dir), nil
}
