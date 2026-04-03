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

func mergeSummary(old, new map[string]any) map[string]any {
	arrayFields := []string{
		"confirmed_needs", "constraints", "excluded_options", "key_data", "current_conclusion",
	}
	for _, field := range arrayFields {
		oldVals := getSlice(old[field])
		newVals := getSlice(new[field])
		vals := make(map[string]struct{}, len(newVals))
		for _, s := range newVals {
			vals[s] = struct{}{}
		}
		for _, s := range oldVals {
			if _, exist := vals[s]; !exist {
				newVals = append(newVals, s)
			}
		}
		new[field] = newVals
	}

	oldVals := getMapSlice(old["discussion_log"])
	newVals := getMapSlice(new["discussion_log"])
	vals := make(map[string]struct{}, len(newVals))
	for _, val := range newVals {
		if t, ok := val["topic"].(string); ok {
			vals[t] = struct{}{}
		}
	}
	for _, val := range oldVals {
		t, ok := val["topic"].(string)
		if !ok {
			continue
		}
		if _, exist := vals[t]; !exist {
			newVals = append(newVals, val)
		}
	}
	new["discussion_log"] = newVals

	return new
}

func getSlice(v any) []string {
	arr, _ := v.([]any)
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func getMapSlice(v any) []map[string]any {
	arr, _ := v.([]any)
	result := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}
