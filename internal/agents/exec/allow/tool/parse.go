// * last modified: 26/11/04
package tool

import (
	"regexp"
	"strings"
)

type toolMeta struct {
	name    *regexp.Regexp
	hasArg  bool
	argGlob *regexp.Regexp
}

/**
 * allow_tool example
 * ---
 * edit_file(/Users/pardn/Desktop/git/Agenvoy/.doc/version-generate/v1.0.23.md)
 * run_command(sh -c echo test > $HOME/always_allow_test.txt && cat $HOME/always_allow_test.txt && rm $HOME/always_allow_test.txt)
 * run_command(sh -c echo test > $HOME/always_allow_test.txt)
 * ...
 */
func parse(line string) (toolMeta, bool) {
	open := strings.IndexByte(line, '(')
	if open < 0 {
		name := strings.TrimSpace(line)
		if name == "" {
			return toolMeta{}, false
		}
		return toolMeta{name: formatRegex(name)}, true
	}
	if !strings.HasSuffix(line, ")") {
		return toolMeta{}, false
	}

	name := strings.TrimSpace(line[:open])
	pattern := line[open+1 : len(line)-1]
	if name == "" {
		return toolMeta{}, false
	}
	return toolMeta{
		name:    formatRegex(name),
		hasArg:  true,
		argGlob: formatRegex(pattern),
	}, true
}

func formatRegex(pattern string) *regexp.Regexp {
	var sb strings.Builder
	sb.WriteByte('^')
	i := 0
	for i < len(pattern) {
		char := pattern[i]
		switch char {
		case '\\':
			if i+1 < len(pattern) {
				next := pattern[i+1]
				// a\*b => a\*b, a\? => a\?, C:\\dir => C:\\dir
				if next == '*' || next == '?' || next == '\\' {
					sb.WriteByte('\\')
					sb.WriteByte(next)
					i += 2
					continue
				}
			}
			// a\nb => a\\nb
			sb.WriteString(`\\`)

		case '*':
			// *.go => [^/]*\.go
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// src/**/*.go => src/(?:.*/)?[^/]*\.go
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					sb.WriteString("(?:.*/)?")
					i += 3
					continue
				}
				// src/** => src/.*
				sb.WriteString(".*")
				i += 2
				continue
			}
			sb.WriteString("[^/]*")

		case '?':
			// file?.txt => file[^/]\.txt
			sb.WriteString("[^/]")

		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']':
			// f(1)+[x] => f\(1\)\+\[x\]
			sb.WriteByte('\\')
			sb.WriteByte(char)

		default:
			// go-mod => go-mod
			sb.WriteByte(char)
		}
		i++
	}
	sb.WriteByte('$')

	re, err := regexp.Compile(sb.String())
	if err != nil {
		// "\xff" (invalid UTF-8) => ^$a (never matches)
		return regexp.MustCompile("^$a")
	}
	return re
}
