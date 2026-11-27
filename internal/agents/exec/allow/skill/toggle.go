// * last modified: 26/11/02
package skill

import (
	"path/filepath"
	"sort"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

// add/remove form list
func Toggle(skillName string) (bool, error) {
	name := strings.TrimSpace(skillName)
	if name == "" {
		return false, utilsmsg.New(utilsmsg.Invalid, "skill name", "empty")
	}

	dic := read(trustSkillPath())
	added := false
	if dic[name] {
		delete(dic, name)
	} else {
		dic[name] = true
		added = true
	}
	return added, write(filesystem.TrustSkillPath, dic)
}

// update list in local
func write(path string, allowMap map[string]bool) error {
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "github.com/pardnchiu/go-pkg/filesystem > CheckDir", filepath.Dir(path), err)
	}

	lines := make([]string, 0, len(allowMap))
	for name := range allowMap {
		lines = append(lines, name)
	}
	sort.Strings(lines)

	/**
	 * trust_skill example
	 * ---
	 * commit-generate
	 * version-generate
	 * ...
	 */
	var sb strings.Builder
	for _, name := range lines {
		sb.WriteString(name)
		sb.WriteByte('\n')
	}
	if err := go_pkg_filesystem.WriteFile(path, sb.String(), 0644); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "github.com/pardnchiu/go-pkg/filesystem > WriteFile", path, err)
	}
	return nil
}

func Revoke(skillName string) (bool, error) {
	name := strings.TrimSpace(skillName)
	if name == "" {
		return false, utilsmsg.New(utilsmsg.Invalid, "skill name", "empty")
	}

	dic := read(trustSkillPath())
	if !dic[name] {
		return false, nil
	}
	delete(dic, name)
	return true, write(filesystem.TrustSkillPath, dic)
}
