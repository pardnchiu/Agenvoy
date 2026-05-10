package exec

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type allowListRule struct {
	toolName *regexp.Regexp
	hasArg   bool
	argGlob  *regexp.Regexp
}

func loadAllowList(workDir string) []allowListRule {
	if strings.TrimSpace(workDir) == "" {
		return nil
	}
	path := filesystem.AllowListPath(workDir)
	if !go_pkg_filesystem_reader.Exists(path) {
		return nil
	}
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return nil
	}
	rules := make([]allowListRule, 0, 16)
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

func parseAllowListEntry(entry string) (allowListRule, bool) {
	open := strings.IndexByte(entry, '(')
	if open < 0 {
		name := strings.TrimSpace(entry)
		if name == "" {
			return allowListRule{}, false
		}
		return allowListRule{toolName: globToRegex(name)}, true
	}
	if !strings.HasSuffix(entry, ")") {
		return allowListRule{}, false
	}
	name := strings.TrimSpace(entry[:open])
	pattern := entry[open+1 : len(entry)-1]
	if name == "" {
		return allowListRule{}, false
	}
	return allowListRule{
		toolName: globToRegex(name),
		hasArg:   true,
		argGlob:  globToRegex(pattern),
	}, true
}

func matchAllowList(rules []allowListRule, toolName, toolArgs string) bool {
	if len(rules) == 0 {
		return false
	}
	canonical := canonicalToolArgs(toolName, toolArgs)
	for _, r := range rules {
		if !r.toolName.MatchString(toolName) {
			continue
		}
		if !r.hasArg {
			return true
		}
		if r.argGlob.MatchString(canonical) {
			return true
		}
	}
	return false
}

func appendAllowListRule(workDir, toolName, toolArgs string) error {
	if strings.TrimSpace(workDir) == "" {
		return fmt.Errorf("empty workDir")
	}
	if strings.TrimSpace(toolName) == "" {
		return fmt.Errorf("empty toolName")
	}
	canonical := canonicalToolArgs(toolName, toolArgs)
	var entry string
	if canonical == "" {
		entry = toolName
	} else {
		entry = toolName + "(" + escapeGlob(canonical) + ")"
	}
	if err := go_pkg_filesystem.CheckDir(filesystem.AllowListDir(workDir), true); err != nil {
		return fmt.Errorf("CheckDir: %w", err)
	}
	path := filesystem.AllowListPath(workDir)
	if go_pkg_filesystem_reader.Exists(path) {
		text, err := go_pkg_filesystem.ReadText(path)
		if err == nil {
			for line := range strings.SplitSeq(text, "\n") {
				if strings.TrimSpace(line) == entry {
					return nil
				}
			}
		}
	}
	if err := go_pkg_filesystem.AppendText(path, entry+"\n"); err != nil {
		return fmt.Errorf("AppendText: %w", err)
	}
	return nil
}

func canonicalToolArgs(toolName, rawArgs string) string {
	rawArgs = strings.TrimSpace(rawArgs)
	if rawArgs == "" {
		return ""
	}
	switch toolName {
	case "run_command":
		var p struct {
			Argv []string `json:"argv"`
		}
		if err := json.Unmarshal([]byte(rawArgs), &p); err == nil && len(p.Argv) > 0 {
			return strings.Join(p.Argv, " ")
		}
	}
	var generic map[string]any
	if err := json.Unmarshal([]byte(rawArgs), &generic); err == nil {
		for _, key := range []string{"path", "url", "file", "target", "command"} {
			if v, ok := generic[key].(string); ok && v != "" {
				return v
			}
		}
	}
	return rawArgs
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

func escapeGlob(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '*', '?', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
