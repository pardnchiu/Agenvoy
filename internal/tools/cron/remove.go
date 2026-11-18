package cron

import (
	"context"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func removeSchedule(ctx context.Context, e *tooltypes.Executor, target, skillName string) (string, error) {
	var removed int
	var err error

	switch target {
	case "task":
		removed, err = runtime.RemoveTask(skillName)
	case "cron":
		removed, err = runtime.RemoveCron(skillName)
	default:
		return "", utilsmsg.New(utilsmsg.Invalid, "target", "must be 'task' or 'cron' (got %q)", target)
	}
	if err != nil {
		return "", err
	}
	if removed == 0 {
		return fmt.Sprintf("no %s found for skill %q", target, skillName), nil
	}
	if err := skill.TrashSchedule(ctx, skillName, historystore.Meta{SessionID: e.SessionID, TaskID: e.PendingTask}); err != nil {
		return "", utilsmsg.Trace("skill.TrashSchedule", skillName, err)
	}
	return fmt.Sprintf("removed %d %s(s) for skill %q and moved skill to .Trash", removed, target, skillName), nil
}
