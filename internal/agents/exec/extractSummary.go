package exec

import (
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/sessionManager"
)

var (
	trailingJsonRegex  = regexp.MustCompile(`(?s)\n*(?:---\s*\n)?(?:\*{0,2}[^\n*]*[Ss]ummary[^\n*]*\*{0,2}\s*\n)?` + "```" + `(?:json)?\s*(\{.*?\})\s*` + "```" + `\s*$`)
	timestampHeaderReg = regexp.MustCompile(`(?m)^-{3,}\n.*\n-{3,}\n`)
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

func extractSummary(sessionID, value string) string {
	const summaryStart = "<summary>"
	const summaryEnd = "</summary>"

	slog.Info("content",
		slog.String("original", value))

	value = timestampHeaderReg.ReplaceAllString(value, "")

	var jsonData any
	var cleaned string

	start := strings.Index(value, summaryStart)
	end := strings.Index(value, summaryEnd)
	if start != -1 && end != -1 && end > start {
		jsonPart := strings.TrimSpace(value[start+len(summaryStart) : end])
		json.Unmarshal([]byte(jsonPart), &jsonData)
		cleaned = strings.TrimRight(value[:start], " \t\n\r")
	} else {
		if start != -1 {
			cleaned = strings.TrimRight(value[:start], " \t\n\r")
		}

		if loc := trailingJsonRegex.FindStringSubmatchIndex(value); loc != nil {
			jsonPart := value[loc[2]:loc[3]]
			var m map[string]any
			if json.Unmarshal([]byte(jsonPart), &m) == nil && isSummaryJSON(m) {
				jsonData = m
				cleaned = strings.TrimRight(value[:loc[0]], " \t\n\r")
			}
		}
		if cleaned == "" {
			cleaned = value
		}
	}

	if jsonData == nil {
		jsonData = map[string]any{
			"confirmed_needs":    []any{},
			"constraints":        []any{},
			"core_discussion":    "",
			"current_conclusion": []any{},
			"discussion_log": []any{
				map[string]any{
					"conclusion": "resolved",
					"time":       time.Now().Format("2006-01-02 15:04"),
					"topic":      strings.TrimSpace(cleaned),
				},
			},
			"excluded_options":  []any{},
			"key_data":          []any{},
			"pending_questions": []any{},
		}
	}

	if newMap, ok := jsonData.(map[string]any); ok {
		_, oldMap := sessionManager.GetSummary(sessionID)
		if oldMap != nil {
			newMap = mergeSummary(oldMap, newMap)
		}
		jsonData = newMap
	}
	sessionManager.SaveSummary(sessionID, jsonData)
	return cleaned
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

	// { "conclusion": "resolved", "time": "2026-02-27 23:57", "topic": "DGX Spark vs Ryzen Halo 比較" },
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
