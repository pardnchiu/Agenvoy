package variant

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func writeSkillFile(ctx context.Context, e *tooltypes.Executor, path, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "content", "required when mode=write")
	}

	absPath, err := skillPath(path)
	if err != nil {
		return "", err
	}

	change := capture(absPath)
	if err := go_pkg_filesystem.WriteFile(absPath, content, 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: WriteFile", absPath, err)
	}
	record(ctx, e, change, content, "edit_skill")

	return fmt.Sprintf("created: %s", absPath), nil
}

func skillPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "path", "required")
	}
	absPath := filepath.Clean(filepath.Join(filesystem.SkillsDir, path))
	if !strings.HasPrefix(absPath, filesystem.SkillsDir+string(filepath.Separator)) {
		return "", utilsmsg.New(utilsmsg.Denied, "path", "must stay within skills dir")
	}
	return absPath, nil
}
