package allowSkill

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func write(path string, set map[string]bool) error {
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return fmt.Errorf("CheckDir: %w", err)
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, name := range names {
		sb.WriteString(name)
		sb.WriteByte('\n')
	}
	if err := go_pkg_filesystem.WriteFile(path, sb.String(), 0644); err != nil {
		return fmt.Errorf("WriteFile: %w", err)
	}
	return nil
}

func ToggleGlobal(name string) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("empty skill name")
	}
	set := LoadGlobal()
	added := false
	if set[name] {
		delete(set, name)
	} else {
		set[name] = true
		added = true
	}
	return added, write(filesystem.AllowSkillGlobalPath, set)
}

func ToggleProject(workDir, name string) (bool, error) {
	if strings.TrimSpace(workDir) == "" {
		return false, fmt.Errorf("empty workDir")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("empty skill name")
	}
	set := LoadProject(workDir)
	added := false
	if set[name] {
		delete(set, name)
	} else {
		set[name] = true
		added = true
	}
	return added, write(filesystem.AllowSkillProjectPath(workDir), set)
}
