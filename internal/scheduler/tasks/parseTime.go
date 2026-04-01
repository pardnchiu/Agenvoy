package tasks

import (
	"fmt"
	"strings"
	"time"
)

func parseTime(text string) (time.Time, error) {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "+") {
		duration, err := time.ParseDuration(text[1:])
		if err != nil {
			return time.Time{}, fmt.Errorf("time.ParseDuration: %w", err)
		}
		return time.Now().Add(duration), nil
	}

	if t, err := time.ParseInLocation("2006-01-02 15:04", text, time.Local); err == nil {
		return t, nil
	}

	if t, err := time.Parse(time.RFC3339, text); err == nil {
		return t, nil
	}

	// * 15:04, no date, assume today
	if t, err := time.ParseInLocation("15:04", text, time.Local); err == nil {
		now := time.Now()
		result := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		if !result.After(now) {
			return time.Time{}, fmt.Errorf("already gone: %q", text)
		}
		return result, nil
	}

	return time.Time{}, fmt.Errorf("parseTime: %s", text)
}
