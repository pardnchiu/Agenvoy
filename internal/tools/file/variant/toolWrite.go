package variant

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func toolTarget(name, tag string, mustExist bool) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", utilsmsg.New(utilsmsg.Invalid, "name", "invalid tool name: %q", name)
	}

	switch tag {
	case "json", "script":
		dir := filepath.Join(filesystem.ScriptToolsDir, name)
		if mustExist && !go_pkg_filesystem_reader.IsDir(dir) {
			return "", utilsmsg.New(utilsmsg.NotFound, "tool", "%q does not exist", name)
		}
		if tag == "json" {
			return filepath.Join(dir, "tool.json"), nil
		}
		return filepath.Join(dir, "script.py"), nil
	}
	return "", utilsmsg.New(utilsmsg.Invalid, "tag", "must be 'json' or 'script', got %q", tag)
}

func writeToolFile(ctx context.Context, e *tooltypes.Executor, name, tag, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "content", "required when mode=write")
	}

	target, err := toolTarget(name, tag, false)
	if err != nil {
		return "", err
	}

	change := capture(target)
	if err := go_pkg_filesystem.WriteFile(target, content, 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteFile", target, err)
	}
	record(ctx, e, change, content, "edit_tool")

	return fmt.Sprintf("created: %s", target), nil
}
