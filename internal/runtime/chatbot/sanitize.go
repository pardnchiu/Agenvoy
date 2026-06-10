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
	htmlTagRegex    = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9-]*)([^>]*)>`)
	allowedTagRegex = regexp.MustCompile(`<(/?)(?i)(b|strong|i|em|u|ins|s|strike|del|code|pre|a|blockquote|tg-spoiler|tg-emoji|span)(\s[^>]*)?>`)
)

func SanitizeTelegramHTML(s string) string {
	s = htmlTagRegex.ReplaceAllStringFunc(s, func(tag string) string {
		m := htmlTagRegex.FindStringSubmatch(tag)
		if m == nil {
			return ""
		}
		if allowedTags[strings.ToLower(m[2])] {
			return tag
		}
		return ""
	})

	locs := allowedTagRegex.FindAllStringIndex(s, -1)
	var b strings.Builder
	b.Grow(len(s))
	prev := 0
	for _, loc := range locs {
		escapeLtGt(&b, s[prev:loc[0]])
		b.WriteString(s[loc[0]:loc[1]])
		prev = loc[1]
	}
	escapeLtGt(&b, s[prev:])
	return b.String()
}

func escapeLtGt(b *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteByte(s[i])
		}
	}
}
