// * last modified: 26/11/02
package skill

import (
	"log/slog"
	"os"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

// check is in list or not
func Match(skillName string) bool {
	name := strings.TrimSpace(skillName)
	if name == "" {
		return false
	}
	return Load()[name]
}

func Load() map[string]bool {
	return read(trustSkillPath())
}

func trustSkillPath() string {
	path, legacy := filesystem.TrustSkillPath, filesystem.LegacyAllowSkillPath
	if go_pkg_filesystem_reader.Exists(path) || !go_pkg_filesystem_reader.Exists(legacy) {
		return path
	}
	if err := os.Rename(legacy, path); err != nil {
		slog.Warn("[Internal] os: Rename",
			slog.String("from", legacy),
			slog.String("to", path),
			slog.String("error", err.Error()))
		return legacy
	}
	return path
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
	 * trust_skill example
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
