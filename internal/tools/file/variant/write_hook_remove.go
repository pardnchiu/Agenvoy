package variant

import (
	"context"
	"fmt"
	"path/filepath"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func writeHookRemove(ctx context.Context, e *tooltypes.Executor, name string) (string, error) {
	absPath, err := hookPath(name, "")
	if err != nil {
		return "", err
	}
	if !go_pkg_filesystem_reader.IsFile(absPath) {
		return "", utilsmsg.New(utilsmsg.NotFound, "hook", "%q does not exist", name)
	}

	trashPath, err := filesystem.TrashDir(absPath, filesystem.HookTrashDir, filepath.Base(absPath))
	if err != nil {
		return "", err
	}
	recordRemoval(ctx, e, absPath, trashPath, "write_hook")

	return fmt.Sprintf("trashed: %s", absPath), nil
}
