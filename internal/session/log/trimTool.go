package sessionLog

import (
	"strconv"
	"strings"
)

func TrimSettledTools(content string) string {
	if content == "" {
		return content
	}

	settled := map[string]bool{}
	lastText := map[string]int{}
	turns := map[string]int{}
	index := 0
	for line := range strings.SplitSeq(content, "\n") {
		m := lineRegex.FindStringSubmatch(line)
		if len(m) < 6 || m[4] == "" {
			index++
			continue
		}
		switch m[3] {
		case "done", "canceled", "error":
			settled[m[4]] = true
		case "agent_result":
			turns[m[4]]++
		case "assistant":
			lastText[m[4]+"#"+strconv.Itoa(turns[m[4]])] = index
		}
		index++
	}
	if len(settled) == 0 {
		return content
	}

	list := make([]string, 0, index)
	clear(turns)
	index = 0
	for line := range strings.SplitSeq(content, "\n") {
		m := lineRegex.FindStringSubmatch(line)
		if len(m) >= 6 && m[4] != "" && m[3] == "agent_result" {
			turns[m[4]]++
		}
		if len(m) >= 6 && settled[m[4]] {
			switch {
			case strings.HasPrefix(m[3], "tool_"), m[3] == "thinking":
				index++
				continue
			case m[3] == "assistant" && lastText[m[4]+"#"+strconv.Itoa(turns[m[4]])] != index:
				index++
				continue
			}
		}
		list = append(list, line)
		index++
	}
	return strings.Join(list, "\n")
}
