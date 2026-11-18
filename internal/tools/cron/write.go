package cron

import (
	"fmt"
	"strings"
	"time"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func writeSchedule(e *tooltypes.Executor, target, when, skill string) (string, error) {
	if !go_pkg_filesystem_reader.Exists(filesystem.ScheduleSkillPath(skill)) {
		return "", utilsmsg.New(utilsmsg.NotFound, "skill", "%q not found under %s. mode=write is an internal binding called by the scheduler-skill-creator skill flow. Run scheduler-skill-creator skill which generates a hashed skill name and binds time in one flow. Do not call mode=write with a hand-made name", skill, filesystem.ScheduleSkillPath(skill))
	}

	switch target {
	case "task":
		at, err := ParseTime(when)
		if err != nil {
			return "", err
		}
		if !at.After(time.Now()) {
			return "", utilsmsg.New(utilsmsg.Invalid, "time", "already gone: %s", at.Local().Format("2006-01-02 15:04:05"))
		}
		entry := runtime.TaskEntry{
			At:        at.UTC(),
			SessionID: strings.TrimSpace(e.SessionID),
			Skill:     skill,
		}
		if err := runtime.AppendTask(entry); err != nil {
			return "", err
		}
		return fmt.Sprintf("task scheduled: %s fires at %s",
			skill, at.Local().Format("2006-01-02 15:04:05")), nil

	case "cron":
		expression := strings.TrimSpace(when)
		if err := ValidateCron(expression); err != nil {
			return "", err
		}
		entry := runtime.CronEntry{
			Expression: expression,
			SessionID:  strings.TrimSpace(e.SessionID),
			Skill:      skill,
		}
		if err := runtime.AppendCron(entry); err != nil {
			return "", err
		}
		return fmt.Sprintf("cron scheduled: %s for %s", expression, skill), nil
	}
	return "", utilsmsg.New(utilsmsg.Invalid, "target", "must be 'task' or 'cron' (got %q)", target)
}
