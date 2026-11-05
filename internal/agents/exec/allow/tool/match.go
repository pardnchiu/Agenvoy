// * last modified: 26/11/04
package tool

import (
	"encoding/json"
	"maps"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

// check is in list or not
func Match(workDir, toolName, toolArgs string) bool {
	dic := Load(workDir)
	if len(dic) == 0 {
		return false
	}

	args, err := formatArgs(workDir, toolName, toolArgs)
	if err != nil {
		return false
	}
	for e := range dic {
		tool, ok := parse(e)
		if !ok || !tool.name.MatchString(toolName) {
			continue
		}
		if !tool.hasArg || tool.argGlob.MatchString(args) {
			return true
		}
	}
	return false
}

// load global and project conbime to one
func Load(workDir ...string) map[string]bool {
	dic := read(filesystem.AllowToolGlobalPath)
	if len(workDir) > 0 && strings.TrimSpace(workDir[0]) != "" {
		maps.Copy(dic, read(filesystem.AllowToolPath(go_pkg_utils.AbsPath("", workDir[0]))))
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

	for line := range strings.SplitSeq(text, "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		dic[entry] = true
	}
	return dic
}

/**
 * allow_tool example
 * ---
 * edit_file(/Users/pardn/Desktop/git/Agenvoy/.doc/version-generate/v1.0.23.md)
 * run_command(sh -c echo test > $HOME/always_allow_test.txt && cat $HOME/always_allow_test.txt && rm $HOME/always_allow_test.txt)
 * run_command(sh -c echo test > $HOME/always_allow_test.txt)
 * ...
 */
func formatArgs(workDir, toolName, toolArgs string) (string, error) {
	args := strings.TrimSpace(toolArgs)
	dir := workDir
	if strings.TrimSpace(dir) == "" {
		dir = filesystem.DownloadDir
	}
	if v, ok, err := formatToolArgs(dir, toolName, args); ok || err != nil {
		return v, err
	}

	if args == "" {
		return "", nil
	}

	var generic map[string]any
	if err := json.Unmarshal([]byte(args), &generic); err == nil {
		for _, key := range []string{"path", "url", "target"} {
			v, ok := generic[key].(string)
			if !ok || strings.TrimSpace(v) == "" {
				continue
			}
			if key == "path" {
				return go_pkg_utils.AbsPath(dir, v), nil
			}
			return v, nil
		}
	}
	return args, nil
}

func formatToolArgs(workDir, toolName, toolArgs string) (string, bool, error) {
	switch toolName {
	case "edit_file":
		var e struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(toolArgs), &e); err != nil {
			return "", true, utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "edit_file", err)
		}

		// ! can not allow edit_file with no path
		path := strings.TrimSpace(e.Path)
		if path == "" {
			return "", true, utilsmsg.New(utilsmsg.Invalid, "edit_file path", "empty; allow rules only target a file path")
		}

		return go_pkg_utils.AbsPath(workDir, path), true, nil

	case "run_command", "run_command_background":
		var e struct {
			Argv []string `json:"argv"`
		}
		if err := json.Unmarshal([]byte(toolArgs), &e); err != nil {
			return "", true, utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "run_command", err)
		}

		// ! can not allow run_command with no args
		if len(e.Argv) == 0 {
			return "", true, utilsmsg.New(utilsmsg.Invalid, "run_command argv", "empty; allow rules only target a command")
		}
		return strings.Join(e.Argv, " "), true, nil

	case "http_request":
		var e struct {
			URL    string `json:"url"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal([]byte(toolArgs), &e); err != nil {
			return "", true, utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "http_request", err)
		}

		// ! can not allow http_request with no url
		url := strings.TrimSpace(e.URL)
		if url == "" {
			return "", true, utilsmsg.New(utilsmsg.Invalid, "http_request url", "empty; allow rules only target a method and url")
		}

		method := strings.ToUpper(strings.TrimSpace(e.Method))
		if method == "" {
			method = "GET"
		}
		return method + " " + url, true, nil
	}
	return "", false, nil
}
