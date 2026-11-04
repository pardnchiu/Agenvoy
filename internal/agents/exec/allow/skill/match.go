// * last modified: 26/11/02
package skill

import (
	"maps"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

// check is in list or not
func Match(workDir, skillName string) bool {
	name := strings.TrimSpace(skillName)
	if name == "" {
		return false
	}
	return Load(workDir)[name]
}

// load global and project conbime to one
func Load(workDir ...string) map[string]bool {
	dic := read(filesystem.AllowSkillGlobalPath)
	if len(workDir) > 0 && strings.TrimSpace(workDir[0]) != "" {
		maps.Copy(dic, read(filesystem.AllowSkillProjectPath(workDir[0])))
	}
	return dic
}

// read path and parse to dic
func read(path string) map[string]bool {
	dic := make(map[string]bool)
	if !go_pkg_filesystem_reader.Exists(path) {
		return dic
	}

	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return dic
	}

	/**
	 * allow_skill example
	 * ---
	 * commit-generate
	 * version-generate
	 * ...
	 */
	for line := range strings.SplitSeq(text, "\n") {
		entry := strings.TrimSpace(line)
		// split by line and trim, skip empty and comment
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		dic[entry] = true
	}
	return dic
}
