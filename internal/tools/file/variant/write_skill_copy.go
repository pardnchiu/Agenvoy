package variant

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var skipCopyNames = map[string]bool{
	".git":        true,
	"__pycache__": true,
	".DS_Store":   true,
}

func writeSkillCopy(e *tooltypes.Executor, from, name string) (string, error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "from", "required when mode=copy")
	}
	src, err := boundary.ResolveRead(e.SessionID, boundary.BaseDir(e.WorkDir), from)
	if err != nil {
		return "", err
	}
	if !go_pkg_filesystem_reader.IsFile(filepath.Join(src, "SKILL.md")) {
		return "", utilsmsg.New(utilsmsg.Invalid, "from", "%s is not a skill directory (no SKILL.md)", src)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(src)
	}
	if err := filesystem.CheckDirName("name", name); err != nil {
		return "", err
	}
	dst := filepath.Join(filesystem.SkillsDir, name)
	if go_pkg_filesystem_reader.Exists(dst) {
		return "", utilsmsg.New(utilsmsg.Conflict, "skill", "%q already exists; remove it with mode=remove first or pick another name", name)
	}

	var copied int
	var skipped []string
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		if skipCopyNames[d.Name()] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || filesystem.IsDeniedPath(path) || filesystem.IsSensitivePath(path) {
			skipped = append(skipped, rel)
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if err := go_pkg_filesystem.Copy(path, filepath.Join(dst, rel)); err != nil {
			return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: Copy", rel, err)
		}
		copied++
		return nil
	})
	if err != nil {
		os.RemoveAll(dst)
		return "", err
	}

	out := fmt.Sprintf("copied: %s → %s (%d files)", src, dst, copied)
	if len(skipped) > 0 {
		out += fmt.Sprintf("\nskipped (symlink or sensitive): %s", strings.Join(skipped, ", "))
	}
	return out + revokeSkillTrust(e, filepath.Join(dst, "SKILL.md")), nil
}
