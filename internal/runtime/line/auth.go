package line

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/utils"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

var pending = utils.NewPendingRegistry[string, string]()

func authorizeSource(target, name string) error {
	path := filesystem.LineAuthPath
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir: %w", err)
	}
	line := target
	if name = strings.TrimSpace(strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(name)); name != "" {
		line = target + "-" + name
	}
	if err := go_pkg_filesystem.AppendText(path, line+"\n"); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem AppendText: %w", err)
	}
	return nil
}
