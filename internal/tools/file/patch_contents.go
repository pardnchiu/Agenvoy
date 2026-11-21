// * last modified: 26/11/21
package file

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/configs"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type patchContent struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
	index      int
}

func patchContents(ctx context.Context, e *tooltypes.Executor, targetPath string, patchContents []patchContent) (string, error) {
	if len(patchContents) == 0 {
		return "", utilsmsg.New(utilsmsg.Invalid, "patch_contents", "required when mode=patch")
	}

	absPath, err := boundary.ResolveIn(e.SessionID, e.WorkDir, targetPath)
	if err != nil {
		return "", utilsmsg.Trace("tools/file/boundary: ResolveIn", "", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os: Stat", "", err)
	}
	if info.Size() > configs.MAX_DOCUMENT_BYTES {
		return "", utilsmsg.New(utilsmsg.Limit, "file", "too large (%d bytes, max %d MiB)", info.Size(), configs.MAX_DOCUMENT_BYTES>>20)
	}
	if err := requireFresh(e, absPath, info.ModTime()); err != nil {
		return "", err
	}

	// NOTE: 1. get file snapshot
	content, err := go_pkg_filesystem.ReadText(absPath)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: ReadText", "", err)
	}
	change := historystore.CaptureContent(absPath, content)

	// NOTE: 2. verify anchors exist
	if err := verifyAnchors(content, patchContents, absPath); err != nil {
		return "", err
	}

	pending := make([]patchContent, 0, len(patchContents))
	for i, e := range patchContents {
		if e.OldString == e.NewString {
			continue
		}
		e.index = i
		pending = append(pending, e)
	}
	if len(pending) == 0 {
		return fmt.Sprintf("no write: every entry in %s already matches its new_string, so the file is unchanged", absPath), nil
	}

	// NOTE: 3. plan patch
	spans, err := planTargets(content, pending, absPath)
	if err != nil {
		return "", err
	}

	// NOTE: 4. apply patch
	oldContent := content
	for _, span := range slices.Backward(spans) {
		content = content[:span.start] + span.text + content[span.end:]
	}

	// NOTE: 5. ensuring content is edited
	if strings.TrimSpace(content) == "" && strings.TrimSpace(oldContent) != "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "patch_contents", "every entry applied but %s would be left empty (it holds %d bytes); nothing written — re-read the file and narrow the anchors, or call write_file with mode=write if emptying it is the intent", absPath, len(oldContent))
	}

	// NOTE: 6. write file content
	if err := go_pkg_filesystem.WriteFile(absPath, content, 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: WriteFile", "", err)
	}

	// NOTE: 7. record file before after
	markWritten(e, absPath)
	e.RecordFile(absPath)

	var unrecorded string
	// NOTE: 8. record action for loop memory => show in the loop done to remind user what files edited in this loop
	if err := historystore.Record(ctx, change, historystore.Meta{SessionID: e.SessionID, TaskID: e.PendingTask, Tool: "write_file"}); err != nil {
		slog.Debug("[Internal] runtime/store: Record",
			slog.String("path", absPath),
			slog.String("error", err.Error()))
		unrecorded = fmt.Sprintf("\nthe previous version was not recorded (%v), so this edit cannot be undone", err)
	}

	return fmt.Sprintf("successfully updated %s", absPath) + unrecorded, nil
}

// NOTE: ensuring patch contents exist in the content
func verifyAnchors(content string, targets []patchContent, absPath string) error {
	for i, target := range targets {
		label := fmt.Sprintf("patch_contents[%d]", i)
		if target.OldString == "" {
			return utilsmsg.New(utilsmsg.Invalid, label, "old_string is required")
		}

		if strings.Contains(content, target.OldString) {
			continue
		}

		// NOTE: ensuring “ ” ‘ ’ " ' wont break anchor; rewrite the quotes to match the actual content
		if actual, ok := findCurlyAnchor(content, target.OldString); ok {
			targets[i].OldString = actual
			targets[i].NewString = curlQuotes(actual, target.NewString)
			continue
		}

		old := strings.TrimRight(target.OldString, "\n")
		for j := range i {
			// NOTE: any not found return error
			if old != "" && strings.Contains(targets[j].NewString, old) {
				return utilsmsg.New(utilsmsg.Invalid, label, "%q also occurs inside patch_contents[%d].new_string. Every anchor resolves against the bytes already on disk, so this entry cannot reach text another entry inserts. Nothing was written — re-read the file and rebuild every entry from its current bytes, merging entries that depend on each other into one", old, j)
			}
		}
		return utilsmsg.Trace(label, "", anchorNotFound(content, target.OldString, absPath))
	}
	return nil
}

func anchorNotFound(content, old, absPath string) error {
	head := ""
	for line := range strings.SplitSeq(old, "\n") {
		if strings.TrimSpace(line) != "" {
			head = strings.TrimSpace(line)
			break
		}
	}

	lines := strings.Split(content, "\n")
	var near []string
	if head != "" {
		for i, line := range lines {
			if strings.TrimSpace(line) == head || strings.Contains(line, head) {
				near = append(near, fmt.Sprintf("row %d is %q", i+1, line))
				if len(near) == 5 {
					break
				}
			}
		}
	}

	if len(near) == 0 {
		return utilsmsg.New(utilsmsg.NotFound, "old_string", "%q is not found in %s and nothing there resembles it; the file holds %d lines — re-read it and build the anchor from its current bytes", old, absPath, len(lines))
	}
	return utilsmsg.New(utilsmsg.NotFound, "old_string", "%q is not found in %s, but %s — copy the anchor from those exact bytes, whitespace included", old, absPath, strings.Join(near, "; "))
}
