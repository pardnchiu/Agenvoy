package variant

import (
	"fmt"
	"os"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

func Register() {
	registWriteTool()
	registPatchTool()
	registTestTool()
	registRemoveTool()
	registWriteSkill()
	registPatchSkill()
	registRemoveSkill()
}

func patch(path, old, new string, replaceAll bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("os.Stat [%s]: %w", path, err)
	}
	if info.Size() > 1<<20 {
		return fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
	}

	content, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem: ReadText [%s]: %w", path, err)
	}

	if !strings.Contains(content, old) {
		return fmt.Errorf("%s is not found in %s", old, path)
	}

	search := old
	if new == "" && !strings.HasSuffix(old, "\n") && strings.Contains(content, old+"\n") {
		search = old + "\n"
	}
	var updated string
	if replaceAll {
		updated = strings.ReplaceAll(content, search, new)
	} else {
		updated = strings.Replace(content, search, new, 1)
	}

	if err := go_pkg_filesystem.WriteFile(path, updated, 0644); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem: WriteFile [%s]: %w", path, err)
	}
	return nil
}
