package telegram

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const telegramChunkBytes = 3200

var tagRegex = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9-]*)([^>]*)>`)

type openTag struct {
	name  string
	attrs string
}

func chunk(str string) []string {
	if len(str) <= telegramChunkBytes {
		return []string{str}
	}
	var chunks []string
	var carry []openTag
	pos := 0
	for pos < len(str) {
		remaining := str[pos:]
		prefix := openTags(carry)
		budget := telegramChunkBytes - len(prefix)
		if budget <= 0 {
			budget = telegramChunkBytes / 2
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
		stack := scanOpenTags(full)
		chunks = append(chunks, full+closeTags(stack))
		carry = stack

		pos += idx
		for pos < len(str) && (str[pos] == '\n' || str[pos] == ' ') {
			pos++
		}
	}
	return chunks
}

func scanOpenTags(s string) []openTag {
	var stack []openTag
	matches := tagRegex.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		closing := m[1] == "/"
		name := strings.ToLower(m[2])
		attrs := m[3]
		if isVoidTag(name) {
			continue
		}
		if !closing {
			stack = append(stack, openTag{name: name, attrs: attrs})
			continue
		}
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].name == name {
				stack = append(stack[:i], stack[i+1:]...)
				break
			}
		}
	}
	return stack
}

func openTags(stack []openTag) string {
	var sb strings.Builder
	for _, t := range stack {
		sb.WriteString("<")
		sb.WriteString(t.name)
		sb.WriteString(t.attrs)
		sb.WriteString(">")
	}
	return sb.String()
}

func closeTags(stack []openTag) string {
	var sb strings.Builder
	for i := len(stack) - 1; i >= 0; i-- {
		sb.WriteString("</")
		sb.WriteString(stack[i].name)
		sb.WriteString(">")
	}
	return sb.String()
}

func isVoidTag(name string) bool {
	switch name {
	case "br", "hr", "img", "input", "meta", "link":
		return true
	}
	return false
}
