package utils

import (
	"fmt"
	"html"
	"strings"
	"unicode/utf16"
)

type ConfirmMessage struct {
	Title           string
	Display         string
	RestrictedLines []string
	RestrictedNote  string
}

const (
	discordMessageLimit  = 2000
	telegramMessageLimit = 4096
	chatRestrictedNote   = "chat cannot collect it: approve it from the TUI or Web UI on this machine"
)

func FormatConfirm(name, raw string, restricted []string, passwordCached bool) ConfirmMessage {
	msg := ConfirmMessage{
		Title:   fmt.Sprintf("Run %s?", ToolName(name)),
		Display: FormatToolEvent(name, raw),
	}
	if msg.Display == "" {
		msg.Display = raw
	}
	if len(restricted) == 0 {
		return msg
	}

	msg.Title = fmt.Sprintf("Run %s outside the allow list?", ToolName(name))
	msg.RestrictedLines = make([]string, len(restricted))
	for i, one := range restricted {
		msg.RestrictedLines[i] = "⚠ " + one
	}
	msg.RestrictedNote = "system password required"
	if passwordCached {
		msg.RestrictedNote = "sudo credentials still valid — no password needed"
	}
	return msg
}

func FormatDiscordConfirm(msg ConfirmMessage, explain string) string {
	explain = chatExplain(msg, explain)
	multiline := strings.Contains(msg.Display, "\n")
	compose := func(display string) string {
		var str string
		if multiline {
			str = fmt.Sprintf("%s\n```\n%s\n```", msg.Title, display)
		} else {
			str = fmt.Sprintf("%s\n`%s`", msg.Title, display)
		}
		if explain != "" {
			str += "\n" + explain
		}
		return str
	}

	str := compose(msg.Display)
	if over := utf16Len(str) - discordMessageLimit; over > 0 {
		str = compose(truncateUTF16(msg.Display, max(utf16Len(msg.Display)-over, 0)))
	}
	return str
}

func FormatTelegramConfirm(msg ConfirmMessage, explain string) (text string, isHTML bool) {
	explain = chatExplain(msg, explain)
	plain := func(display string) string {
		str := fmt.Sprintf("%s\n\n%s", msg.Title, display)
		if explain != "" {
			str += "\n\n" + explain
		}
		return str
	}

	if over := utf16Len(plain(msg.Display)) - telegramMessageLimit; over > 0 {
		return plain(truncateUTF16(msg.Display, max(utf16Len(msg.Display)-over, 0))), false
	}

	body := fmt.Sprintf("<code>%s</code>", html.EscapeString(msg.Display))
	if strings.Contains(msg.Display, "\n") {
		body = fmt.Sprintf("<pre><code>%s</code></pre>", html.EscapeString(msg.Display))
	}
	text = fmt.Sprintf("%s\n\n%s", html.EscapeString(msg.Title), body)
	if explain != "" {
		text += "\n\n" + html.EscapeString(explain)
	}
	return text, true
}

func chatExplain(msg ConfirmMessage, explain string) string {
	if len(msg.RestrictedLines) == 0 {
		return explain
	}
	return strings.TrimSpace(explain + "\n" + strings.Join(msg.RestrictedLines, "\n") + "\n" + msg.RestrictedNote + "\n" + chatRestrictedNote)
}

func utf16Len(text string) int {
	return len(utf16.Encode([]rune(text)))
}

func truncateUTF16(text string, limit int) string {
	if utf16Len(text) <= limit {
		return text
	}
	units := 0
	for i, r := range text {
		n := max(utf16.RuneLen(r), 1)
		if units+n > limit-3 {
			return text[:i] + "..."
		}
		units += n
	}
	return text
}
