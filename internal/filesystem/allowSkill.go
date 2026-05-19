package filesystem

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

const AllowSkillRelPath = ".agenvoy/allow_skill"
const AllowSkillFileName = "allow_skill"

func AllowSkillProjectPath(workDir string) string {
	return filepath.Join(workDir, AllowSkillRelPath)
}

func AllowSkillProjectDir(workDir string) string {
	return filepath.Join(workDir, filepath.Dir(AllowSkillRelPath))
}

func AllowSkillGlobalPath() string {
	return filepath.Join(AgenvoyDir, AllowSkillFileName)
}

func readAllowSkill(path string) map[string]bool {
	out := make(map[string]bool)
	if !go_pkg_filesystem_reader.Exists(path) {
		return out
	}
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return out
	}
	for line := range strings.SplitSeq(text, "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		out[entry] = true
	}
	return out
}

func writeAllowSkill(path string, set map[string]bool) error {
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

func LoadAllowSkillGlobal() map[string]bool {
	return readAllowSkill(AllowSkillGlobalPath())
}

func LoadAllowSkillProject(workDir string) map[string]bool {
	if strings.TrimSpace(workDir) == "" {
		return map[string]bool{}
	}
	return readAllowSkill(AllowSkillProjectPath(workDir))
}

func LoadAllowSkillEffective(workDir string) map[string]bool {
	out := LoadAllowSkillGlobal()
	for name := range LoadAllowSkillProject(workDir) {
		out[name] = true
	}
	return out
}

func IsSkillAllowed(workDir, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if LoadAllowSkillGlobal()[name] {
		return true
	}
	if strings.TrimSpace(workDir) != "" && LoadAllowSkillProject(workDir)[name] {
		return true
	}
	return false
}

func ToggleAllowSkillGlobal(name string) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("empty skill name")
	}
	set := LoadAllowSkillGlobal()
	added := false
	if set[name] {
		delete(set, name)
	} else {
		set[name] = true
		added = true
	}
	return added, writeAllowSkill(AllowSkillGlobalPath(), set)
}

func ToggleAllowSkillProject(workDir, name string) (bool, error) {
	if strings.TrimSpace(workDir) == "" {
		return false, fmt.Errorf("empty workDir")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("empty skill name")
	}
	set := LoadAllowSkillProject(workDir)
	added := false
	if set[name] {
		delete(set, name)
	} else {
		set[name] = true
		added = true
	}
	return added, writeAllowSkill(AllowSkillProjectPath(workDir), set)
}
