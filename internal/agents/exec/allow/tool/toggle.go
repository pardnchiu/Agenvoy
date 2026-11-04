// * last modified: 26/11/04
package tool

import (
	"path/filepath"
	"sort"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

// add/remove form list
func Toggle(toolName, toolArgs string, workDir ...string) (bool, error) {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return false, utilsmsg.New(utilsmsg.Invalid, "tool name", "empty")
	}

	// default is global, if assign path, then path
	dir := ""
	if len(workDir) > 0 {
		if strings.TrimSpace(workDir[0]) == "" {
			return false, utilsmsg.New(utilsmsg.Invalid, "work dir", "empty")
		}
		dir = go_pkg_utils.AbsPath("", workDir[0])
	}

	path := filesystem.AllowToolGlobalPath
	if dir != "" {
		path = filesystem.AllowToolPath(dir)
	}

	subject, err := formatArgs(dir, name, toolArgs)
	if err != nil {
		return false, err
	}

	entry := name
	if subject != "" {
		entry = name + "(" + formatGlob(subject) + ")"
	}

	dic := read(path)
	added := false
	if dic[entry] {
		delete(dic, entry)
	} else {
		dic[entry] = true
		added = true
	}
	return added, write(path, dic)
}

func ToggleMCP(prefix string, entries []string) error {
	// not mcp, then skip
	if strings.TrimSpace(prefix) == "" {
		return utilsmsg.New(utilsmsg.Invalid, "prefix", "empty")
	}

	set := Load()
	for entry := range set {
		if strings.HasPrefix(entry, prefix) {
			delete(set, entry)
		}
	}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry != "" {
			set[entry] = true
		}
	}
	return write(filesystem.AllowToolGlobalPath, set)
}

func write(path string, allowMap map[string]bool) error {
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "github.com/pardnchiu/go-pkg/filesystem > CheckDir", filepath.Dir(path), err)
	}

	lines := make([]string, 0, len(allowMap))
	for entry := range allowMap {
		lines = append(lines, entry)
	}
	sort.Strings(lines)

	/**
	 * allow_tool example
	 * ---
	 * edit_file(/Users/pardn/Desktop/git/Agenvoy/.doc/version-generate/v1.0.23.md)
	 * run_command(sh -c echo test > $HOME/always_allow_test.txt && cat $HOME/always_allow_test.txt && rm $HOME/always_allow_test.txt)
	 * run_command(sh -c echo test > $HOME/always_allow_test.txt)
	 * ...
	 */
	var sb strings.Builder
	for _, entry := range lines {
		sb.WriteString(entry)
		sb.WriteByte('\n')
	}
	if err := go_pkg_filesystem.WriteFile(path, sb.String(), 0644); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "github.com/pardnchiu/go-pkg/filesystem > WriteFile", path, err)
	}
	return nil
}

func formatGlob(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '*', '?', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}
