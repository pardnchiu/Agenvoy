package summary

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
)

func GetPrompt(sessionID string, cutoff time.Time) string {
	raw, summaryMap := Get(sessionID)
	if raw == nil {
		return ""
	}

	if !cutoff.IsZero() && summaryMap != nil {
		filter(summaryMap, "past_discussions", "last_discussed", cutoff)
		filter(summaryMap, "discussion_log", "time", cutoff)
		if b, err := json.Marshal(summaryMap); err == nil {
			raw = b
		}
	}

	return strings.NewReplacer(
		"{{.Summary}}", string(raw),
	).Replace(strings.TrimSpace(configs.SummaryContext))
}

func filter(summaryMap map[string]any, listField, timeField string, cutoff time.Time) {
	items, ok := summaryMap[listField].([]any)
	if !ok {
		return
	}

	list := make([]any, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, ok := record[timeField].(string)
		if !ok {
			list = append(list, item)
			continue
		}
		parsed, err := time.ParseInLocation("2006-01-02 15:04", t, time.Local)
		if err != nil {
			list = append(list, item)
			continue
		}
		if !parsed.Before(cutoff) {
			list = append(list, item)
		}
	}
	summaryMap[listField] = list
}
