package line

import (
	"strings"
	"unicode/utf8"
)

const (
	lineChunkBytes = 4400
)

func chunk(text string) []string {
	if len(text) <= lineChunkBytes {
		return []string{text}
	}
	var chunks []string
	pos := 0
	for pos < len(text) {
		remaining := text[pos:]
		if len(remaining) <= lineChunkBytes {
			chunks = append(chunks, remaining)
			break
		}

		window := remaining[:lineChunkBytes]
		idx := strings.LastIndex(window, "\n")
		if idx < 0 {
			idx = strings.LastIndex(window, " ")
		}
		if idx <= 0 {
			idx = lineChunkBytes
			for idx > 0 && !utf8.RuneStart(remaining[idx]) {
				idx--
			}
		}

		chunks = append(chunks, strings.TrimRight(remaining[:idx], " \n"))
		pos += idx
		for pos < len(text) && (text[pos] == '\n' || text[pos] == ' ') {
			pos++
		}
	}
	return chunks
}
