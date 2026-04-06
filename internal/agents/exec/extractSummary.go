package exec

import (
	"regexp"
)

var (
	fencedBlockRegex    = regexp.MustCompile("(?s)" + "```" + `(?:json|summary)\s*\n([\s\S]*?)\s*\n` + "```")
	summaryTagRegex     = regexp.MustCompile(`(?s)<summary>\s*([\s\S]*?)\s*</summary>`)
	summaryBracketRegex = regexp.MustCompile(`(?s)\[summary\]\s*([\s\S]*?)\s*\[/summary\]`)
)

func isSummaryJSON(m map[string]any) bool {
	keys := []string{
		"core_discussion", "discussion_log", "confirmed_needs", "current_conclusion",
	}
	matched := 0
	for _, key := range keys {
		if _, exist := m[key]; exist {
			matched++
		}
	}
	return matched >= 2
}

