package interactive

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const BackgroundTool = "subagents"

var (
	updateNumberRegex = regexp.MustCompile(`report #(\d+)`)
	updateElidedRegex = regexp.MustCompile(`^\.\.\.\[(\d+) reports elided(?:: #(\d+)-#(\d+))?\]\.\.\.$`)
)

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
		return
	}
	backgroundSlotCounts[sessionID]--
}

type BackgroundTask struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	WorkDir     string   `json:"work_dir,omitempty"`
	Done        bool     `json:"done,omitempty"`
	Reported    bool     `json:"reported,omitempty"`
	Result      string   `json:"result,omitempty"`
	Updates     []string `json:"updates,omitempty"`
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
	unregisterBackgroundCancel(sessionID, taskHash, id)
	if takeBackgroundCanceled(sessionID, taskHash) {
		return ErrBackgroundCanceled
	}
	return finishBackground(sessionID, taskHash, id, result)
}

func finishBackground(sessionID, taskHash, id, result string) error {
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

func UpdateBackground(sessionID, taskHash, id, text string) error {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return utilsmsg.Trace("readPending", taskHash, err)
	}
	i := slices.IndexFunc(meta.Background, func(b BackgroundTask) bool { return b.ID == id })
	if i < 0 || meta.Background[i].Done {
		return utilsmsg.New(utilsmsg.NotFound, "background task", "%q running in pending %q", id, taskHash)
	}
	meta.Background[i].Updates = trimUpdates(append(meta.Background[i].Updates, clampPendingText(text, configs.MAX_PENDING_RESULT_BYTES/4)), configs.MAX_PENDING_RESULT_BYTES)
	return writePending(sessionID, taskHash, &meta)
}

func trimUpdates(list []string, budget int) []string {
	total := 0
	for _, one := range list {
		total += len(one)
	}
	if total <= budget || len(list) < 3 {
		return list
	}

	first, rest := list[0], list[1:]
	count, low, high := 0, 0, 0
	if m := updateElidedRegex.FindStringSubmatch(rest[0]); m != nil {
		count, _ = strconv.Atoi(m[1])
		low, _ = strconv.Atoi(m[2])
		high, _ = strconv.Atoi(m[3])
		rest = rest[1:]
	}

	left := budget - len(first) - 64
	cut := len(rest)
	for cut > 0 && (cut == len(rest) || left-len(rest[cut-1]) >= 0) {
		left -= len(rest[cut-1])
		cut--
	}
	if cut == 0 && count == 0 {
		return list
	}

	for _, one := range rest[:cut] {
		count++
		if m := updateNumberRegex.FindStringSubmatch(one); m != nil {
			n, _ := strconv.Atoi(m[1])
			if low == 0 || n < low {
				low = n
			}
			high = max(high, n)
		}
	}
	marker := fmt.Sprintf("...[%d reports elided]...", count)
	if low > 0 {
		marker = fmt.Sprintf("...[%d reports elided: #%d-#%d]...", count, low, high)
	}
	return append([]string{first, marker}, rest[cut:]...)
}

func BackgroundState(sessionID, taskHash string) (running, unreported int) {
	if taskHash == "" {
		return 0, 0
	}
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return 0, 0
	}
	finished, running := backgroundToReport(meta.Background)
	return running, len(finished)
}

func LoadPendingWindow(sessionID, taskHash string) string {
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return ""
	}
	return meta.WindowHash
}

func ClaimFollowUp(sessionID, taskHash string) (string, bool) {
	if taskHash == "" || !HasBackground(sessionID, taskHash) {
		return "", false
	}
	return taskHash, !ClaimOnline(sessionID, taskHash)
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
			if len(b.Updates) > 0 {
				finished = append(finished, b)
			}
		case !b.Reported:
			finished = append(finished, b)
		}
	}
	return finished, running
}

func backgroundResumeHeader(finished []BackgroundTask) string {
	if slices.ContainsFunc(finished, func(b BackgroundTask) bool { return b.Done }) {
		return "[Resumed Task — background task finished]"
	}
	return "[Resumed Task — background task update]"
}

func formatBackgroundSection(finished []BackgroundTask, running int) string {
	var sb strings.Builder
	sb.WriteString("\n## Background Task Reports\n")
	for _, b := range finished {
		if !b.Done {
			fmt.Fprintf(&sb, "\n### %s · %s (still running, update)\n%s\n", b.ID, b.Description, strings.Join(b.Updates, "\n\n"))
			continue
		}
		if len(b.Updates) > 0 {
			fmt.Fprintf(&sb, "\n### %s · %s\n%s\n\n%s\n", b.ID, b.Description, strings.Join(b.Updates, "\n\n"), b.Result)
			continue
		}
		fmt.Fprintf(&sb, "\n### %s · %s\n%s\n", b.ID, b.Description, b.Result)
	}
	if running > 0 {
		fmt.Fprintf(&sb, "\n%d background task(s) still running. Handle the finished results above; this task stays pending and resumes again as each remaining one finishes.\n", running)
	} else {
		sb.WriteString("\nAll background tasks of this task have finished.\n")
	}
	return sb.String()
}

type stagedReport struct {
	history  string
	finished []BackgroundTask
}

var (
	stagedMu      sync.Mutex
	stagedReports = map[string]stagedReport{}
)

func stageBackgroundReport(sessionID, taskHash, history string, finished []BackgroundTask) {
	stagedMu.Lock()
	defer stagedMu.Unlock()
	key := sessionID + ":" + taskHash
	if len(finished) == 0 {
		delete(stagedReports, key)
		return
	}
	stagedReports[key] = stagedReport{history: strings.TrimSpace(history), finished: finished}
}

func CommitBackgroundReport(sessionID, taskHash, history string) {
	key := sessionID + ":" + taskHash
	stagedMu.Lock()
	staged, ok := stagedReports[key]
	if ok && staged.history == strings.TrimSpace(history) {
		delete(stagedReports, key)
	} else {
		ok = false
	}
	stagedMu.Unlock()
	if ok {
		markBackgroundReported(sessionID, taskHash, staged.finished)
	}
}

func markBackgroundReported(sessionID, taskHash string, finished []BackgroundTask) {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return
	}
	for i, b := range meta.Background {
		j := slices.IndexFunc(finished, func(f BackgroundTask) bool { return f.ID == b.ID })
		if j < 0 {
			continue
		}
		if finished[j].Done {
			meta.Background[i].Reported = true
		}
		meta.Background[i].Updates = slices.DeleteFunc(meta.Background[i].Updates, func(one string) bool {
			return slices.Contains(finished[j].Updates, one)
		})
	}
	if err := writePending(sessionID, taskHash, &meta); err != nil {
		slog.Debug("markBackgroundReported",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
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
		if err := os.RemoveAll(filesystem.BackgroundDir(entry.Name())); err != nil {
			slog.Warn("ClearAllBackground",
				slog.String("session", entry.Name()),
				slog.String("error", err.Error()))
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
