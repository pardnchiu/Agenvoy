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

func writeToolRemove(ctx context.Context, e *tooltypes.Executor, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", utilsmsg.New(utilsmsg.Invalid, "name", "invalid tool name: %q", name)
	}

	dir := filepath.Join(filesystem.ScriptToolsDir, name)
	if !go_pkg_filesystem_reader.IsDir(dir) {
		return "", utilsmsg.New(utilsmsg.NotFound, "tool", "%q does not exist", name)
	}

	trashPath, err := filesystem.TrashDir(dir, filesystem.ScriptToolTrashDir, name)
	if err != nil {
		return "", err
	}
	recordRemoval(ctx, e, dir, trashPath, "write_tool")

	return fmt.Sprintf("trashed: %s", dir), nil
}
