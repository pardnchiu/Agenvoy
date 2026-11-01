package interactive

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const BackgroundTool = "subagents"

var (
	backgroundSlotMu     sync.Mutex
	backgroundSlotCounts = map[string]int{}
)

func AcquireBackgroundSlot(sessionID string) error {
	backgroundSlotMu.Lock()
	defer backgroundSlotMu.Unlock()

	if backgroundSlotCounts[sessionID] >= configs.MAX_BACKGROUND_TASKS {
		return utilsmsg.New(utilsmsg.Limit, "background", "%d background tasks are already running in this session; run this one without background or wait for one to finish", configs.MAX_BACKGROUND_TASKS)
	}
	backgroundSlotCounts[sessionID]++
	return nil
}

func ReleaseBackgroundSlot(sessionID string) {
	backgroundSlotMu.Lock()
	defer backgroundSlotMu.Unlock()

	if backgroundSlotCounts[sessionID] <= 1 {
		delete(backgroundSlotCounts, sessionID)
	} else {
		backgroundSlotCounts[sessionID]--
	}
}

func IsBackgroundTool(name string) bool {
	return name == BackgroundTool || name == "run_command"
}

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

func backgroundRunning(path string) bool {
	meta, err := readPending(path)
	return err == nil && slices.ContainsFunc(meta.Background, func(b BackgroundTask) bool { return !b.Done })
}

func ClearAllBackground() {
	entries, err := os.ReadDir(filesystem.SessionsDir)
	if err != nil {
		return
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		for _, taskHash := range listPendingTasks(entry.Name()) {
			meta, err := readPending(filesystem.PendingMetaPath(entry.Name(), taskHash))
			if err != nil || len(meta.Background) == 0 {
				continue
			}
			meta.Background = nil
			if err := writePending(entry.Name(), taskHash, &meta); err != nil {
				slog.Warn("ClearAllBackground",
					slog.String("session", entry.Name()),
					slog.String("task", taskHash),
					slog.String("error", err.Error()))
			}
		}
	}
}
