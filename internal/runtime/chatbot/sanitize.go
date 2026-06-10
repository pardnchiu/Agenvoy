package chatbot

import (
	"regexp"
	"strings"
)

var (
	allowedTags = map[string]bool{
		"b": true, "strong": true,
		"i": true, "em": true,
		"u": true, "ins": true,
		"s": true, "strike": true, "del": true,
		"code": true, "pre": true,
		"a":          true,
		"blockquote": true,
		"tg-spoiler": true, "tg-emoji": true,
		"span": true,
	}
	htmlTagRegex = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9-]*)([^>]*)>`)
)

func SanitizeTelegramHTML(s string) string {
	return htmlTagRegex.ReplaceAllStringFunc(s, func(tag string) string {
		m := htmlTagRegex.FindStringSubmatch(tag)
		if m == nil {
			return ""
		}
		name := strings.ToLower(m[2])
		if allowedTags[name] {
			return tag
		}
		return ""
	})
}
