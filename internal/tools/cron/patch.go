package cron

import (
	"fmt"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func patchSchedule(target, when, skill string) (string, error) {
	switch target {
	case "task":
		at, err := ParseTime(when)
		if err != nil {
			return "", err
		}
		if !at.After(time.Now()) {
			return "", utilsmsg.New(utilsmsg.Invalid, "time", "already gone: %s", at.Local().Format("2006-01-02 15:04:05"))
		}
		patched, err := runtime.PatchTask(skill, at.UTC())
		if err != nil {
			return "", err
		}
		if patched == 0 {
			return fmt.Sprintf("no task found for skill %q", skill), nil
		}
		return fmt.Sprintf("patched %d task(s) for skill %q; new fire time: %s",
			patched, skill, at.Local().Format("2006-01-02 15:04:05")), nil

	case "cron":
		expression := strings.TrimSpace(when)
		if err := ValidateCron(expression); err != nil {
			return "", err
		}
		patched, err := runtime.PatchCron(skill, expression)
		if err != nil {
			return "", err
		}
		if patched == 0 {
			return fmt.Sprintf("no cron found for skill %q", skill), nil
		}
		return fmt.Sprintf("patched %d cron(s) for skill %q; new expression: %s",
			patched, skill, expression), nil
	}
	return "", utilsmsg.New(utilsmsg.Invalid, "target", "must be 'task' or 'cron' (got %q)", target)
}
