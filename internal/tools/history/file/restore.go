package fileHistory

import (
	"context"
	"fmt"
	"strings"

	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Restore(ctx context.Context, e *tooltypes.Executor, version int64, taskID0 string, paths []string) (string, error) {
	taskID := strings.TrimSpace(taskID0)
	if taskID == "current" {
		taskID = e.PendingTask
	}

	meta := historystore.Meta{
		SessionID: e.SessionID,
		TaskID:    e.PendingTask,
		Tool:      "restore_file",
	}

	switch {
	case version > 0:
		out, err := historystore.RestoreTo(ctx, version, meta)
		if err != nil {
			return "", utilsmsg.Trace("historystore.RestoreTo", fmt.Sprintf("%d", version), err)
		}
		return out, nil
	case taskID == "":
		return "", utilsmsg.New(utilsmsg.Invalid, "version", "version or task_id is required")
	}

	filters := []historystore.Filter{{TaskID: taskID}}
	if len(paths) > 0 {
		filters = filters[:0]
		for _, path := range paths {
			filters = append(filters, historystore.Filter{TaskID: taskID, Path: path})
		}
	}

	var report []string
	for _, filter := range filters {
		lines, err := historystore.Undo(ctx, filter, meta)
		if err != nil {
			return "", utilsmsg.Trace("historystore.Undo", "", err)
		}
		if len(lines) == 0 {
			if filter.Path != "" {
				report = append(report, fmt.Sprintf("%s was not changed by task %s", filter.Path, taskID))
			}
			continue
		}
		report = append(report, lines...)
	}
	if len(report) == 0 {
		return fmt.Sprintf("task %s changed no files, so there is nothing to undo", taskID), nil
	}
	return strings.Join(report, "\n"), nil
}
