package readonly

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
)

func IsRunCommand(toolArgs string) bool {
	var p struct {
		Argv    []string `json:"argv"`
		Network bool     `json:"network"`
	}
	if json.Unmarshal([]byte(toolArgs), &p) != nil || len(p.Argv) == 0 || p.Network {
		return false
	}
	if script, ok := shellScript(p.Argv); ok {
		return isReadOnlyShell(script)
	}
	return isReadOnlyArgv(p.Argv)
}

func isReadOnlyArgv(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	bin := filepath.Base(argv[0])
	if bin == "git" {
		var ok bool
		if argv, ok = stripSafeGitGlobalFlags(argv); !ok {
			return false
		}
	}

	matched := ""
	for _, candidate := range readOnlyCandidates(bin, argv) {
		if slices.Contains(filesystem.ReadOnlyCommand, candidate) {
			matched = candidate
			break
		}
	}
	if matched == "" {
		return false
	}
	if slices.ContainsFunc(argv[1:], boundary.IsSensitivePath) {
		return false
	}
	if bin == "git" {
		return !hasDangerousGitFlag(argv[2:]) && !hasGitRefMutation(argv)
	}
	if exactArgvOnly(matched) && len(strings.Fields(matched)) != len(argv) {
		return false
	}
	if check, ok := readOnlyArgCheck[bin]; ok {
		return check(argv[1:])
	}
	return true
}

func readOnlyCandidates(bin string, argv []string) []string {
	list := make([]string, 0, 3)
	if len(argv) > 2 {
		list = append(list, bin+" "+argv[1]+" "+argv[2])
	}
	if len(argv) > 1 {
		list = append(list, bin+" "+argv[1])
	}
	return append(list, bin)
}

var infoFlags = []string{"--version", "-v", "-V", "-version", "-m"}

var infoFlagWithArgs = []string{"unzip -v", "command -v", "command -V"}

func exactArgvOnly(entry string) bool {
	fields := strings.Fields(entry)
	return len(fields) > 1 && slices.Contains(infoFlags, fields[len(fields)-1]) && !slices.Contains(infoFlagWithArgs, entry)
}

func shortFlagHas(a, letters string) bool {
	return strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a[1:], letters)
}

var hostnameReadFlags = []string{
	"-s", "--short", "-f", "--fqdn", "--long", "-d", "--domain",
	"-i", "--ip-address", "-I", "--all-ip-addresses", "-A", "--all-fqdns",
	"-a", "--alias", "-y", "--yp", "--nis", "-V", "--version", "-h", "--help",
}

var readOnlyArgCheck = map[string]func(args []string) bool{
	"date": func(args []string) bool {
		return !slices.ContainsFunc(args, func(a string) bool {
			if strings.HasPrefix(a, "+") || strings.HasPrefix(a, "-I") {
				return false
			}
			return !strings.HasPrefix(a, "-") || strings.HasPrefix(a, "--set") || shortFlagHas(a, "s")
		})
	},
	"hostname": func(args []string) bool {
		return !slices.ContainsFunc(args, func(a string) bool { return !slices.Contains(hostnameReadFlags, a) })
	},
	"sysctl": func(args []string) bool {
		return !slices.ContainsFunc(args, func(a string) bool {
			return a == "--write" || strings.HasPrefix(a, "--load") || a == "--system" || strings.Contains(a, "=") || shortFlagHas(a, "wpf")
		})
	},
	"nm":      rejectPluginFlag,
	"objdump": rejectPluginFlag,
}

func rejectPluginFlag(args []string) bool {
	return !slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "--plugin") })
}

var gitGlobalFlags = []string{
	"--no-pager", "-P", "--no-optional-locks", "--no-replace-objects",
	"--literal-pathspecs", "--glob-pathspecs", "--noglob-pathspecs", "--icase-pathspecs",
}

func stripSafeGitGlobalFlags(argv []string) ([]string, bool) {
	i := 1
	for i < len(argv) && strings.HasPrefix(argv[i], "-") {
		switch {
		case argv[i] == "-C" && i+1 < len(argv):
			i += 2
		case slices.Contains(gitGlobalFlags, argv[i]):
			i++
		default:
			return nil, false
		}
	}
	return append([]string{argv[0]}, argv[i:]...), true
}

func hasDangerousGitFlag(args []string) bool {
	for _, a := range args {
		switch {
		case a == "-o", a == "--output", a == "--output-directory":
			return true
		case strings.HasPrefix(a, "--output=") || strings.HasPrefix(a, "--output-directory="):
			return true
		case strings.HasPrefix(a, "-o") && a != "-o":
			return true
		case a == "--ext-diff", a == "--textconv", strings.HasPrefix(a, "--open-files-in-pager"), strings.HasPrefix(a, "-O"):
			return true
		}
	}
	return false
}

var gitRefMutation = map[string]bool{
	"-d": true, "-D": true, "--delete": true,
	"-m": true, "-M": true, "--move": true,
	"-c": true, "-C": true, "--copy": true,
	"-u": true, "--set-upstream-to": true, "--unset-upstream": true, "--edit-description": true,
	"-f": true, "--force": true,
	"add": true, "rm": true, "remove": true, "rename": true, "set-url": true, "set-head": true, "set-branches": true, "prune": true, "update": true,
}

func hasGitRefMutation(argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	switch argv[1] {
	case "branch", "tag", "remote":
	default:
		return false
	}
	return slices.ContainsFunc(argv[2:], func(a string) bool {
		flag, _, _ := strings.Cut(a, "=")
		return gitRefMutation[flag]
	})
}
