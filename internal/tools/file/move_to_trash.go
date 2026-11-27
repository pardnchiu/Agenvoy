package file

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func MoveToTrash(ctx context.Context, e *tooltypes.Executor, paths []string, tool string) (string, error) {
	var moved, failed, unrecorded []string
	var errs []error
	for _, one := range paths {
		if err := ctx.Err(); err != nil {
			return "", utilsmsg.New(utilsmsg.Canceled, "remove", "cancelled: %w", err)
		}
		if strings.HasPrefix(one, "-") {
			continue
		}

		src, err := boundary.ResolveWrite(e.SessionID, e.WorkDir, one)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", one, err))
			errs = append(errs, err)
			continue
		}

		dst, err := filesystem.MoveToStoreTemp(src)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", one, err))
			errs = append(errs, err)
			continue
		}

		moved = append(moved, src)
		e.RecordFile(src)
		if err := historystore.RecordDelete(ctx, src, dst, historystore.Meta{SessionID: e.SessionID, TaskID: e.PendingTask, Tool: tool}); err != nil {
			slog.Debug("[Internal] runtime/store: RecordDelete",
				slog.String("path", src),
				slog.String("error", err.Error()))
			unrecorded = append(unrecorded, fmt.Sprintf("%s now at %s (%v)", src, dst, err))
		}
	}

	switch {
	case len(moved) == 0 && len(failed) == 0:
		return "", utilsmsg.New(utilsmsg.Invalid, "paths", "required")
	case len(moved) == 0:
		return "", utilsmsg.Trace("remove", "", errors.Join(errs...))
	}

	report := fmt.Sprintf("moved to %s: %s", filesystem.StoreTempDir, strings.Join(moved, ", "))
	if len(failed) > 0 {
		report += "\nnot removed: " + strings.Join(failed, "; ")
	}
	if len(unrecorded) > 0 {
		report += "\nnot recorded, so restore_file will not find these: " + strings.Join(unrecorded, "; ")
	}
	return report, nil
}
