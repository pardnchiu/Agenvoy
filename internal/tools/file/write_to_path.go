// * last modified: 26/11/21
package file

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func writeToPath(ctx context.Context, e *tooltypes.Executor, targetPath, newContent, tool string) (string, error) {
	absPath, err := boundary.ResolveIn(e.SessionID, e.WorkDir, targetPath)
	if err != nil {
		return "", utilsmsg.Trace("tools/file/boundary: ResolveIn", "", err)
	}

	info, err := os.Stat(absPath)
	isNew := os.IsNotExist(err)
	if err != nil && !isNew {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os: Stat", "", err)
	}
	if !isNew && info.Size() > configs.MAX_DOCUMENT_BYTES {
		return "", utilsmsg.New(utilsmsg.Limit, "file", "too large (%d bytes, max %d MiB)", info.Size(), configs.MAX_DOCUMENT_BYTES>>20)
	}
	if !isNew {
		if err := requireFresh(e, absPath, info.ModTime()); err != nil {
			return "", err
		}
	}

	// NOTE: 1. get file snapshot
	change, err := historystore.Capture(absPath)
	if err != nil {
		slog.Debug("runtime/store: Capture",
			slog.String("path", absPath),
			slog.String("error", err.Error()))
	}

	// NOTE: 2. write file content
	if err := go_pkg_filesystem.WriteFile(absPath, newContent, 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: WriteFile", "", err)
	}

	var unrecorded string
	// NOTE: 3 record action for loop memory => show in the loop done to remind user what files edited in this loop
	markWritten(e, absPath)
	e.RecordFile(absPath)

	// NOTE: 4. record file before after
	if err := historystore.Record(ctx, change.WithCreated(newContent), historystore.Meta{
		SessionID: e.SessionID,
		TaskID:    e.PendingTask,
		Tool:      tool,
	}); err != nil {
		slog.Debug("runtime/store: Record",
			slog.String("path", absPath),
			slog.String("error", err.Error()))
		unrecorded = fmt.Sprintf("\nthe previous version was not recorded (%v), so this edit cannot be undone", err)
	}

	return writeReceipt(isNew, absPath, newContent) + unrecorded, nil
}

func writeReceipt(isNew bool, path, content string) string {
	verb := "updated"
	if isNew {
		verb = "created"
	}

	size := len(content)
	if info, err := os.Stat(path); err == nil {
		size = int(info.Size())
	}

	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	lineCount := len(lines)
	if content == "" {
		lineCount = 0
	}
	return fmt.Sprintf(
		"successfully %s: %s (%d bytes, %d lines on disk)\nfirst line: %s\nlast line: %s\nThis is the file's real content. The elided write argument left in history is a context-saving marker, not what was written — do not rewrite the file to \"restore\" it.",
		verb, path, size, lineCount,
		excerptLine(lines[0]), excerptLine(lines[len(lines)-1]),
	)
}

func excerptLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return "(blank)"
	}
	return go_pkg_utils.TruncateString(line, 128)
}
