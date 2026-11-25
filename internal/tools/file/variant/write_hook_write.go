package variant

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	agentsexechook "github.com/pardnchiu/agenvoy/internal/agents/exec/hook"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func writeHookWrite(ctx context.Context, e *tooltypes.Executor, name, lang, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "content", "required when mode=write")
	}

	absPath, err := hookPath(name, lang)
	if err != nil {
		return "", err
	}

	change := capture(absPath)
	if err := go_pkg_filesystem.WriteFile(absPath, content, 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: WriteFile", absPath, err)
	}
	record(ctx, e, change, content, "write_hook")

	return fmt.Sprintf("created: %s", absPath), nil
}

func hookPath(name, lang string) (string, error) {
	name = strings.TrimSpace(name)
	if !slices.Contains(agentsexechook.Names(), name) {
		return "", utilsmsg.New(utilsmsg.Invalid, "name", "unknown hook %q; available: %s", name, strings.Join(agentsexechook.Names(), ", "))
	}
	for _, ext := range []string{".py", ".js"} {
		if path := filepath.Join(filesystem.HooksDir, name+ext); go_pkg_filesystem_reader.IsFile(path) {
			return path, nil
		}
	}
	if strings.TrimSpace(lang) == "js" {
		return filepath.Join(filesystem.HooksDir, name+".js"), nil
	}
	return filepath.Join(filesystem.HooksDir, name+".py"), nil
}
