package allowTool

import (
	"regexp"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type ToolRule struct {
	name    *regexp.Regexp
	hasArg  bool
	argGlob *regexp.Regexp
}

func List(workDir string) []ToolRule {
	if strings.TrimSpace(workDir) == "" {
		return nil
	}
	path := filesystem.AllowToolPath(workDir)
	if !go_pkg_filesystem_reader.Exists(path) {
		return nil
	}
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return nil
	}
	rules := make([]ToolRule, 0, 16)
	for line := range strings.SplitSeq(text, "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		r, ok := parseAllowListEntry(entry)
		if !ok {
			continue
		}
		rules = append(rules, r)
	}
	return rules
}

func parseAllowListEntry(entry string) (ToolRule, bool) {
	open := strings.IndexByte(entry, '(')
	if open < 0 {
		name := strings.TrimSpace(entry)
		if name == "" {
			return ToolRule{}, false
		}
		return ToolRule{name: globToRegex(name)}, true
	}
	if !strings.HasSuffix(entry, ")") {
		return ToolRule{}, false
	}
	name := strings.TrimSpace(entry[:open])
	pattern := entry[open+1 : len(entry)-1]
	if name == "" {
		return ToolRule{}, false
	}
	return ToolRule{
		name:    globToRegex(name),
		hasArg:  true,
		argGlob: globToRegex(pattern),
	}, true
}

func globToRegex(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteByte('^')
	i := 0
	for i < len(pattern) {
		c := pattern[i]
		switch c {
		case '\\':
			if i+1 < len(pattern) {
				next := pattern[i+1]
				if next == '*' || next == '?' || next == '\\' {
					b.WriteByte('\\')
					b.WriteByte(next)
					i += 2
					continue
				}
			}
			b.WriteString(`\\`)
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i += 2
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
		i++
	}
	b.WriteByte('$')
	re, err := regexp.Compile(b.String())
	if err != nil {
		return regexp.MustCompile("^$a")
	}
	return re
}
