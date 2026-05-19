package allowTool

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func Append(workDir, toolName, toolArgs string) error {
	if strings.TrimSpace(workDir) == "" {
		return fmt.Errorf("empty workDir")
	}
	if strings.TrimSpace(toolName) == "" {
		return fmt.Errorf("empty toolName")
	}
	canonical := canonicalToolArgs(toolName, toolArgs)
	var entry string
	if canonical == "" {
		entry = toolName
	} else {
		entry = toolName + "(" + escapeGlob(canonical) + ")"
	}
	path := filesystem.AllowToolPath(workDir)
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return fmt.Errorf("CheckDir: %w", err)
	}
	if go_pkg_filesystem_reader.Exists(path) {
		text, err := go_pkg_filesystem.ReadText(path)
		if err == nil {
			for line := range strings.SplitSeq(text, "\n") {
				if strings.TrimSpace(line) == entry {
					return nil
				}
			}
		}
	}
	if err := go_pkg_filesystem.AppendText(path, entry+"\n"); err != nil {
		return fmt.Errorf("AppendText: %w", err)
	}
	return nil
}

func escapeGlob(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '*', '?', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
