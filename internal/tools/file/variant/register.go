package variant

import (
	"context"
	"log/slog"
	"os"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/configs"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Register() {
	registEditSkill()
	registEditTool()
	registTestTool()
}

func capture(path string) historystore.Change {
	change, err := historystore.Capture(path)
	if err != nil {
		slog.Debug("[Internal] runtime/store: Capture",
			slog.String("path", path),
			slog.String("error", err.Error()))
	}
	return change
}

func record(ctx context.Context, e *tooltypes.Executor, change historystore.Change, created, tool string) {
	if err := historystore.Record(ctx, change.WithCreated(created), metaOf(e, tool)); err != nil {
		slog.Debug("[Internal] runtime/store: Record",
			slog.String("tool", tool),
			slog.String("error", err.Error()))
	}
}

func recordRemoval(ctx context.Context, e *tooltypes.Executor, path, trashPath, tool string) {
	if err := historystore.RecordDelete(ctx, path, trashPath, metaOf(e, tool)); err != nil {
		slog.Debug("[Internal] runtime/store: RecordDelete",
			slog.String("path", path),
			slog.String("error", err.Error()))
	}
}

func metaOf(e *tooltypes.Executor, tool string) historystore.Meta {
	if e == nil {
		return historystore.Meta{Tool: tool}
	}
	return historystore.Meta{SessionID: e.SessionID, TaskID: e.PendingTask, Tool: tool}
}

func patch(path, old, new string, replaceAll bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os: Stat", path, err)
	}
	if info.Size() > configs.MAX_DOCUMENT_BYTES {
		return utilsmsg.New(utilsmsg.Limit, "file", "too large (%d bytes, max %d MiB)", info.Size(), configs.MAX_DOCUMENT_BYTES>>20)
	}

	content, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: ReadText", path, err)
	}

	if !strings.Contains(content, old) {
		return utilsmsg.New(utilsmsg.NotFound, "old_string", "%s is not found in %s", old, path)
	}

	search := old
	if new == "" && !strings.HasSuffix(old, "\n") && strings.Contains(content, old+"\n") {
		search = old + "\n"
	}
	var updated string
	if replaceAll {
		updated = strings.ReplaceAll(content, search, new)
	} else {
		updated = strings.Replace(content, search, new, 1)
	}

	if err := go_pkg_filesystem.WriteFile(path, updated, 0644); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: WriteFile", path, err)
	}
	return nil
}
