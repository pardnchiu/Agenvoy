package summary

import (
	"regexp"
)

var (
	fencedBlockRegex    = regexp.MustCompile("(?s)" + "```" + `(?:json|summary)\s*\n([\s\S]*?)\s*\n` + "```")
	summaryTagRegex     = regexp.MustCompile(`(?s)<summary>\s*([\s\S]*?)\s*</summary>`)
	summaryBracketRegex = regexp.MustCompile(`(?s)\[summary\]\s*([\s\S]*?)\s*\[/summary\]`)
)

func isSummaryJSON(m map[string]any) bool {
	newKeys := []string{"key_decisions", "past_discussions", "current_discussion"}
	if countMatch(m, newKeys) >= 2 {
		return true
	}
	legacyKeys := []string{"core_discussion", "discussion_log", "confirmed_needs", "current_conclusion"}
	return countMatch(m, legacyKeys) >= 2
}

func countMatch(record map[string]any, keys []string) int {
	num := 0
	for _, key := range keys {
		if _, ok := record[key]; ok {
			num++
		}
	}
	return num
}
