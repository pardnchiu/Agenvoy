package discord

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const discordChunkBytes = 1600

var fenceRegex = regexp.MustCompile("```([a-zA-Z0-9_+-]*)")

func chunk(text string) []string {
	if len(text) <= discordChunkBytes {
		return []string{text}
	}
	var chunks []string
	carryLang := ""
	carryOpen := false
	pos := 0
	for pos < len(text) {
		remaining := text[pos:]
		prefix := ""
		if carryOpen {
			prefix = "```" + carryLang + "\n"
		}
		budget := discordChunkBytes - len(prefix)
		if budget <= 0 {
			budget = discordChunkBytes / 2
		}
		if len(remaining) <= budget {
			chunks = append(chunks, prefix+remaining)
			break
		}

		window := remaining[:budget]
		idx := strings.LastIndex(window, "\n\n")
		if idx < 0 {
			idx = strings.LastIndex(window, "\n")
		}
		if idx < 0 {
			idx = strings.LastIndex(window, " ")
		}
		if idx <= 0 {
			idx = budget
			for idx > 0 && !utf8.RuneStart(remaining[idx]) {
				idx--
			}
		}

		body := strings.TrimRight(remaining[:idx], " \n")
		full := prefix + body
		newLang, isOpen := fenceState(full)
		suffix := ""
		if isOpen {
			suffix = "\n```"
		}
		chunks = append(chunks, full+suffix)
		carryLang = newLang
		carryOpen = isOpen

		pos += idx
		for pos < len(text) && (text[pos] == '\n' || text[pos] == ' ') {
			pos++
		}
	}
	return chunks
}

func fenceState(s string) (string, bool) {
	matches := fenceRegex.FindAllStringSubmatch(s, -1)
	if len(matches)%2 == 0 {
		return "", false
	}
	return matches[len(matches)-1][1], true
}
