package cron

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/cron"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

// * allow: +5m, +1h30m, 15:04, 2006-01-02 15:04, RFC3339
func AddOneTimeTask(timeText, script, channelID string) (string, error) {
	mgr := cron.Get()
	if mgr == nil {
		return "", fmt.Errorf("scheduler not initialized")
	}

	at, err := parseTime(timeText)
	if err != nil {
		return "", err
	}

	if err := mgr.AddTask(at, script, channelID); err != nil {
		return "", err
	}

	return fmt.Sprintf("already set up one-time-task at %s: %s",
		at.Local().Format("2006-01-02 15:04:05"), script), nil
}

func parseTime(timeText string) (time.Time, error) {
	timeText = strings.TrimSpace(timeText)

	// * +5m, +1h30m, append to now
	if strings.HasPrefix(timeText, "+") {
		duration, err := time.ParseDuration(timeText[1:])
		if err != nil {
			return time.Time{}, fmt.Errorf("time.ParseDuration: %w", err)
		}
		return time.Now().Add(duration), nil
	}

	if newTime, err := time.ParseInLocation("2006-01-02 15:04", timeText, time.Local); err == nil {
		return newTime, nil
	}

	if newTime, err := time.Parse(time.RFC3339, timeText); err == nil {
		return newTime, nil
	}

	// * 15:04, no date, assume today
	if newHour, err := time.ParseInLocation("15:04", timeText, time.Local); err == nil {
		now := time.Now()
		newTime := time.Date(now.Year(), now.Month(), now.Day(), newHour.Hour(), newHour.Minute(), 0, 0, time.Local)

		// * already gone
		if !newTime.After(now) {
			return time.Time{}, fmt.Errorf("already gone: %q", timeText)
		}
		return newTime, nil
	}

	return time.Time{}, fmt.Errorf("parseTime: %s", timeText)
}

// * save to fixed location: scheduler/scripts
func WriteScript(name, content string) (string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".sh" && ext != ".py" {
		return "", fmt.Errorf("scripts only support .sh or .py")
	}

	if filepath.Base(name) != name {
		return "", fmt.Errorf("must not contain path separator")
	}

	// configDir, err := utils.GetConfigDir("scheduler")
	// if err != nil {
	// 	return "", fmt.Errorf("utils.GetConfigDir: %w", err)
	// }

	scriptsDir := filepath.Join(filesystem.SchedulerDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return "", fmt.Errorf("os.MkdirAll: %w", err)
	}

	base := strings.TrimSuffix(name, ext)
	uniqueName := fmt.Sprintf("%s_%d%s", base, time.Now().UTC().Unix(), ext)

	path := filepath.Join(scriptsDir, uniqueName)
	if err := filesystem.WriteFile(path, content, 0755); err != nil {
		return "", fmt.Errorf("utils.WriteFile: %w", err)
	}

	return fmt.Sprintf(`script saved. pass "%s" as the script parameter to add_onetime_task`, uniqueName), nil
}
