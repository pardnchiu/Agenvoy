package utils

import (
	"html"
	"regexp"
	"strings"
)

var (
	htmlTagRegex       = regexp.MustCompile(`<[^>]+>`)
	sendFileRegex      = regexp.MustCompile(`\[SEND_FILE:[^\]]+\]`)
	sendVoiceRegex     = regexp.MustCompile(`\[SEND_VOICE:[^\]]+\]`)
	discordFooterRegex = regexp.MustCompile(`(?m)^-#.*$`)
	spaceRegex         = regexp.MustCompile(`\s+`)
)

func CleanVoiceReplyText(reply string) string {
	text := strings.TrimSpace(reply)
	if text == "" {
		return ""
	}
	text = sendFileRegex.ReplaceAllString(text, " ")
	text = sendVoiceRegex.ReplaceAllString(text, " ")
	text = discordFooterRegex.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "\u200b", " ")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = htmlTagRegex.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	text = strings.Trim(text, "`*_#>- \n\t\r")
	text = spaceRegex.ReplaceAllString(text, " ")
	for _, p := range []string{"，", "。", "、", "！", "？", "：", "；", ",", ".", "!", "?", ":", ";"} {
		text = strings.ReplaceAll(text, " "+p, p)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return text
}

func VoiceReplyText(reply string) string {
	text := CleanVoiceReplyText(reply)
	if text == "" {
		return ""
	}
	const fullLimit = 320
	if len([]rune(text)) <= fullLimit {
		return text
	}

	return "語音概要：" + broadSummary(text)
}

func broadSummary(text string) string {
	rs := []rune(strings.TrimSpace(text))
	if len(rs) <= 420 {
		return string(rs)
	}

	const headLen = 170
	const middleLen = 120
	const tailLen = 170

	midStart := len(rs)/2 - middleLen/2
	if midStart < headLen {
		midStart = headLen
	}
	if midStart+middleLen > len(rs)-tailLen {
		midStart = len(rs) - tailLen - middleLen
	}
	if midStart < 0 {
		midStart = 0
	}

	head := strings.TrimSpace(string(rs[:headLen]))
	middle := strings.TrimSpace(string(rs[midStart : midStart+middleLen]))
	tail := strings.TrimSpace(string(rs[len(rs)-tailLen:]))
	return strings.Join([]string{head, middle, tail}, " ... ")
}
