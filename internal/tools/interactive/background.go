package interactive

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const BackgroundTool = "subagents"

type BackgroundTask struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	WorkDir     string `json:"work_dir,omitempty"`
	Done        bool   `json:"done,omitempty"`
	Reported    bool   `json:"reported,omitempty"`
	Result      string `json:"result,omitempty"`
}

func AddBackground(sessionID, taskHash, workDir, id, description string) error {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return utilsmsg.Trace("readPending", taskHash, err)
	}
	meta.Background = append(meta.Background, BackgroundTask{ID: id, Description: description, WorkDir: workDir})
	return writePending(sessionID, taskHash, &meta)
}

func FinishBackground(sessionID, taskHash, id, result string) error {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return utilsmsg.Trace("readPending", taskHash, err)
	}
	i := slices.IndexFunc(meta.Background, func(b BackgroundTask) bool { return b.ID == id })
	if i < 0 {
		return utilsmsg.New(utilsmsg.NotFound, "background task", "%q in pending %q", id, taskHash)
	}
	meta.Background[i].Done = true
	meta.Background[i].Result = clampPendingText(result, configs.MAX_PENDING_RESULT_BYTES)
	return writePending(sessionID, taskHash, &meta)
}

func BackgroundState(sessionID, taskHash string) (running, unreported int) {
	if taskHash == "" {
		return 0, 0
	}
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return 0, 0
	}
	for _, b := range meta.Background {
		switch {
		case !b.Done:
			running++
		case !b.Reported:
			unreported++
		}
	}
	return running, unreported
}

func LoadPendingWindow(sessionID, taskHash string) string {
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return ""
	}
	return meta.WindowHash
}

func HasBackground(sessionID, taskHash string) bool {
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	return err == nil && len(meta.Background) > 0
}

func SaveToolResults(sessionID, taskHash, windowHash string, results []ToolResult, files []string) {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return
	}
	meta.Files = mergeFiles(meta.Files, files)
	for _, r := range results {
		if r.ID == "" || slices.ContainsFunc(meta.ToolResults, func(e ToolResult) bool { return e.ID == r.ID }) {
			continue
		}
		meta.ToolResults = append(meta.ToolResults, r)
	}
	meta.ToolAttempts = nil
	meta.WindowHash = windowHash
	if err := writePending(sessionID, taskHash, &meta); err != nil {
		slog.Debug("SaveToolResults",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
}

func backgroundToReport(list []BackgroundTask) (finished []BackgroundTask, running int) {
	for _, b := range list {
		switch {
		case !b.Done:
			running++
		case !b.Reported:
			finished = append(finished, b)
		}
	}
	return finished, running
}

func formatBackgroundSection(finished []BackgroundTask, running int) string {
	var sb strings.Builder
	sb.WriteString("\n## Background Tasks Finished\n")
	for _, b := range finished {
		fmt.Fprintf(&sb, "\n### %s · %s\n%s\n", b.ID, b.Description, b.Result)
	}
	if running > 0 {
		fmt.Fprintf(&sb, "\n%d background task(s) still running. Handle the finished results above; this task stays pending and resumes again as each remaining one finishes.\n", running)
	} else {
		sb.WriteString("\nAll background tasks of this task have finished.\n")
	}
	return sb.String()
}

func markBackgroundReported(sessionID, taskHash string, finished []BackgroundTask) {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return
	}
	for i, b := range meta.Background {
		if slices.ContainsFunc(finished, func(f BackgroundTask) bool { return f.ID == b.ID }) {
			meta.Background[i].Reported = true
		}
	}
	if err := writePending(sessionID, taskHash, &meta); err != nil {
		slog.Debug("markBackgroundReported",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
}

func UnfinishedBackground() []BackgroundTaskRef {
	entries, err := os.ReadDir(filesystem.SessionsDir)
	if err != nil {
		return nil
	}
	var list []BackgroundTaskRef
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		for _, taskHash := range listPendingTasks(entry.Name()) {
			meta, err := readPending(filesystem.PendingMetaPath(entry.Name(), taskHash))
			if err != nil {
				continue
			}
			for _, b := range meta.Background {
				if !b.Done {
					list = append(list, BackgroundTaskRef{SessionID: entry.Name(), TaskHash: taskHash, ID: b.ID, WorkDir: b.WorkDir})
				}
			}
		}
	}
	return list
}

type BackgroundTaskRef struct {
	SessionID string
	TaskHash  string
	ID        string
	WorkDir   string
}
