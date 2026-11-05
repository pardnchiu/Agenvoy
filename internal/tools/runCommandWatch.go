package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/configs"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var watchBinaries = map[string]bool{
	"chokidar": true,
}

var packageRunners = map[string]bool{
	"npm": true, "pnpm": true, "yarn": true, "bun": true,
}

func watchCommandErr(command string) error {
	return utilsmsg.New(utilsmsg.Invalid, "run_command", "%q starts a file watcher that never exits; a foreground call would hang until the %s timeout. Re-run it with run_command_background and timeout_min set to how long it should watch, or run a one-shot build instead (the same tool without --watch, or the package script that builds once)", command, configs.RUN_COMMAND_TIMEOUT)
}

func isWatchFlag(arg string) bool {
	return arg == "--watch" || strings.HasPrefix(arg, "--watch=")
}

func checkWatchArgs(args []string, dir string, depth int, seen map[string]bool) error {
	if len(args) == 0 || depth > configs.MAX_WATCH_SCRIPT_DEPTH {
		return nil
	}

	base := filepath.Base(args[0])
	if watchBinaries[base] || slices.ContainsFunc(args[1:], isWatchFlag) {
		return watchCommandErr(strings.Join(args, " "))
	}
	if (base == "sh" || base == "bash") && len(args) >= 3 && args[1] == "-c" {
		return checkWatchScript(args[2], dir, depth+1, seen)
	}
	if !packageRunners[base] {
		return nil
	}

	name := packageScriptName(args[1:])
	if name == "" {
		return nil
	}
	script, pkgDir := findPackageScript(dir, name)
	if script == "" {
		return nil
	}
	key := filepath.Join(pkgDir, "package.json") + "#" + name
	if seen[key] {
		return nil
	}
	seen[key] = true
	if err := checkWatchScript(script, pkgDir, depth+1, seen); err != nil {
		return utilsmsg.Trace(fmt.Sprintf("%q runs package script %q (%s)", strings.Join(args, " "), name, script), "", err)
	}
	return nil
}

func checkWatchScript(script, dir string, depth int, seen map[string]bool) error {
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return nil
	}

	var bad error
	syntax.Walk(file, func(node syntax.Node) bool {
		if bad != nil {
			return false
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		if _, ok := staticWord(call.Args[0]); !ok {
			return true
		}

		args := make([]string, 0, len(call.Args))
		for _, w := range call.Args {
			if s, ok := staticWord(w); ok {
				args = append(args, s)
			}
		}
		if filepath.Base(args[0]) == "cd" {
			if len(args) > 1 {
				dir = resolveWatchDir(dir, args[1])
			}
			return true
		}
		bad = checkWatchArgs(args, dir, depth, seen)
		return bad == nil
	})
	return bad
}

func packageScriptName(args []string) string {
	var positional []string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
		}
	}
	if len(positional) == 0 {
		return ""
	}
	if positional[0] == "run" || positional[0] == "run-script" {
		if len(positional) < 2 {
			return ""
		}
		return positional[1]
	}
	return positional[0]
}

func findPackageScript(dir, name string) (string, string) {
	for {
		path := filepath.Join(dir, "package.json")
		if go_pkg_filesystem_reader.Exists(path) {
			pkg, err := go_pkg_filesystem.ReadJSON[struct {
				Scripts map[string]string `json:"scripts"`
			}](path)
			if err != nil {
				return "", ""
			}
			return pkg.Scripts[name], dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ""
		}
		dir = parent
	}
}

func resolveWatchDir(dir, target string) string {
	if rest, ok := strings.CutPrefix(target, "~"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return dir
		}
		return filepath.Join(home, rest)
	}
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Join(dir, target)
}
