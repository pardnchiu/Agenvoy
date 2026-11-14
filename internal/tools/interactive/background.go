package interactive

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
	"github.com/pardnchiu/agenvoy/internal/utils/secret"
)

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

type BackgroundGoal struct {
	Objective string
	Next      string
}

type BackgroundTask struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Done        bool     `json:"done,omitempty"`
	Reported    bool     `json:"reported,omitempty"`
	Result      string   `json:"result,omitempty"`
	Updates     []string `json:"updates,omitempty"`
	job         string
}

type BackgroundJob struct {
	ID         string           `json:"id"`
	SessionID  string           `json:"session_id"`
	ParentTask string           `json:"parent_task"`
	Group      string           `json:"group,omitempty"`
	WorkDir    string           `json:"work_dir,omitempty"`
	WindowHash string           `json:"window_hash,omitempty"`
	AllowAll   bool             `json:"allow_all,omitempty"`
	Objective  string           `json:"objective,omitempty"`
	Next       string           `json:"next,omitempty"`
	Legs       []BackgroundTask `json:"legs,omitempty"`
}

var jobMu sync.Mutex

func readJob(path string) (BackgroundJob, error) {
	job, err := go_pkg_filesystem.ReadJSON[BackgroundJob](path)
	if err != nil {
		return BackgroundJob{}, utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.ReadJSON", path, err)
	}
	return job, nil
}

func writeJob(job *BackgroundJob) error {
	dir := filesystem.BackgroundDir(job.SessionID)
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", dir, err)
	}
	if err := go_pkg_filesystem.WriteJSON(filesystem.BackgroundJobPath(job.SessionID, job.ID), job, false); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteJSON", job.ID, err)
	}
	return nil
}

func parentJobs(sessionID, parentTask string) []BackgroundJob {
	if parentTask == "" {
		return nil
	}
	paths, err := filepath.Glob(filepath.Join(filesystem.BackgroundDir(sessionID), "job_*.json"))
	if err != nil {
		return nil
	}
	var list []BackgroundJob
	for _, path := range paths {
		job, err := readJob(path)
		if err != nil || job.ParentTask != parentTask {
			continue
		}
		list = append(list, job)
	}
	return list
}

func addLeg(sessionID, parentTask, group, workDir, windowHash string, goal BackgroundGoal, id, description string) (string, error) {
	jobMu.Lock()
	defer jobMu.Unlock()

	jobs := parentJobs(sessionID, parentTask)
	i := slices.IndexFunc(jobs, func(job BackgroundJob) bool { return group != "" && job.Group == group })
	var job BackgroundJob
	if i >= 0 {
		job = jobs[i]
	} else {
		job = BackgroundJob{
			ID:         "job_" + strings.ReplaceAll(go_pkg_utils.UUID(), "-", "")[:8],
			SessionID:  sessionID,
			ParentTask: parentTask,
			Group:      group,
			WorkDir:    workDir,
			WindowHash: windowHash,
			AllowAll:   LoadPendingAllowAll(sessionID, parentTask),
		}
	}
	if job.Objective == "" {
		job.Objective = strings.TrimSpace(goal.Objective)
	}
	if job.Next == "" {
		job.Next = strings.TrimSpace(goal.Next)
	}
	job.Legs = append(job.Legs, BackgroundTask{ID: id, Description: description})
	if err := writeJob(&job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func FinishBackground(sessionID, jobID, id, result string) (string, error) {
	if takeLegCanceled(id) {
		return "", ErrBackgroundCanceled
	}
	jobMu.Lock()
	job, err := readJob(filesystem.BackgroundJobPath(sessionID, jobID))
	jobMu.Unlock()
	if err != nil {
		return "", utilsmsg.Trace("readJob", jobID, err)
	}
	unregisterBackgroundCancel(sessionID, job.ParentTask, id)
	if takeBackgroundCanceled(sessionID, job.ParentTask) {
		return job.ParentTask, ErrBackgroundCanceled
	}
	return updateLeg(sessionID, jobID, id, func(leg *BackgroundTask) bool {
		leg.Done = true
		leg.Result = clampPendingText(secret.Redact(result), configs.MAX_PENDING_RESULT_BYTES)
		return true
	})
}

func UpdateBackground(sessionID, jobID, id, text string) (string, error) {
	return updateLeg(sessionID, jobID, id, func(leg *BackgroundTask) bool {
		if leg.Done {
			return false
		}
		leg.Updates = trimUpdates(append(leg.Updates, clampPendingText(secret.Redact(text), configs.MAX_PENDING_RESULT_BYTES/4)), configs.MAX_PENDING_RESULT_BYTES)
		return true
	})
}

func updateLeg(sessionID, jobID, id string, apply func(*BackgroundTask) bool) (string, error) {
	jobMu.Lock()
	defer jobMu.Unlock()

	job, err := readJob(filesystem.BackgroundJobPath(sessionID, jobID))
	if err != nil {
		return "", utilsmsg.Trace("readJob", jobID, err)
	}
	i := slices.IndexFunc(job.Legs, func(b BackgroundTask) bool { return b.ID == id })
	if i < 0 || !apply(&job.Legs[i]) {
		return job.ParentTask, utilsmsg.New(utilsmsg.NotFound, "background task", "%q running in job %q", id, jobID)
	}
	return job.ParentTask, writeJob(&job)
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

func RunningBackground(sessionID, parentTask string) []BackgroundTask {
	var list []BackgroundTask
	for _, one := range parentLegs(sessionID, parentTask) {
		if !one.Done {
			list = append(list, one)
		}
	}
	return list
}

func parentLegs(sessionID, parentTask string) []BackgroundTask {
	var list []BackgroundTask
	for _, job := range parentJobs(sessionID, parentTask) {
		for _, leg := range job.Legs {
			leg.job = job.ID
			list = append(list, leg)
		}
	}
	return list
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
	if taskHash == "" || !go_pkg_filesystem_reader.Exists(filesystem.PendingMetaPath(sessionID, taskHash)) {
		return false
	}
	return len(parentJobs(sessionID, taskHash)) > 0
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
		return "[Background Report — background task finished]"
	}
	return "[Background Report — background task update]"
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
		fmt.Fprintf(&sb, "\n%d background task(s) still running. Their reports arrive here while this task is open; once it has ended, each group runs its next step as its own task when all of its legs finish.\n", running)
	} else {
		sb.WriteString("\nAll background tasks of this task have finished.\n")
	}
	return sb.String()
}

func TakeBackgroundReport(sessionID, parentTask string) string {
	if parentTask == "" {
		return ""
	}
	finished, running := backgroundToReport(unstagedLegs(sessionID, parentTask))
	if len(finished) == 0 {
		return ""
	}
	markBackgroundReported(sessionID, finished)
	return backgroundResumeHeader(finished) + "\n" + formatBackgroundSection(finished, running)
}

type stagedReport struct {
	history  string
	finished []BackgroundTask
}

var (
	stagedMu      sync.Mutex
	stagedReports = map[string]stagedReport{}
)

func unstagedLegs(sessionID, parentTask string) []BackgroundTask {
	list := parentLegs(sessionID, parentTask)
	stagedMu.Lock()
	staged := stagedReports[sessionID+":"+parentTask].finished
	stagedMu.Unlock()
	return slices.DeleteFunc(list, func(leg BackgroundTask) bool {
		return slices.ContainsFunc(staged, func(one BackgroundTask) bool { return one.job == leg.job && one.ID == leg.ID })
	})
}

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
		markBackgroundReported(sessionID, staged.finished)
	}
}

func markBackgroundReported(sessionID string, finished []BackgroundTask) {
	jobMu.Lock()
	defer jobMu.Unlock()

	jobIDs := map[string]bool{}
	for _, one := range finished {
		jobIDs[one.job] = true
	}
	for jobID := range jobIDs {
		job, err := readJob(filesystem.BackgroundJobPath(sessionID, jobID))
		if err != nil {
			continue
		}
		for i, b := range job.Legs {
			j := slices.IndexFunc(finished, func(f BackgroundTask) bool { return f.job == jobID && f.ID == b.ID })
			if j < 0 {
				continue
			}
			if finished[j].Done {
				job.Legs[i].Reported = true
			}
			job.Legs[i].Updates = slices.DeleteFunc(job.Legs[i].Updates, func(one string) bool {
				return slices.Contains(finished[j].Updates, one)
			})
		}
		if err := writeJob(&job); err != nil {
			slog.Debug("markBackgroundReported",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}
	}
}

func SettleBackground(sessionID, parentTask string) {
	if parentTask == "" || go_pkg_filesystem_reader.Exists(filesystem.PendingMetaPath(sessionID, parentTask)) {
		return
	}
	jobMu.Lock()
	var ready []BackgroundJob
	for _, job := range parentJobs(sessionID, parentTask) {
		if slices.ContainsFunc(job.Legs, func(leg BackgroundTask) bool { return !leg.Done }) {
			continue
		}
		os.Remove(filesystem.BackgroundJobPath(sessionID, job.ID))
		if slices.ContainsFunc(job.Legs, func(leg BackgroundTask) bool { return !leg.Reported || len(leg.Updates) > 0 }) {
			ready = append(ready, job)
		}
	}
	jobMu.Unlock()

	if backgroundTrigger == nil {
		return
	}
	for _, job := range ready {
		backgroundTrigger(job)
	}
}

func BackgroundJobInput(job BackgroundJob) string {
	var sb strings.Builder
	sb.WriteString("[Resumed Task — background group finished]\n\n## Objective\n")
	if job.Objective != "" {
		sb.WriteString(job.Objective)
	} else {
		sb.WriteString("Not given; take it from the origin task below.")
	}
	sb.WriteString("\n\n## Next Step\n")
	if job.Next != "" {
		sb.WriteString(job.Next)
	} else {
		sb.WriteString("Use the reports below to carry the objective forward and reply with the outcome.")
	}
	fmt.Fprintf(&sb, "\n\n## Origin Task\nStarted by task %s, which has already ended. When the reports need its context, read it with chat_history(mode=read, task_id=[%q]).\n", job.ParentTask, job.ParentTask)
	sb.WriteString(formatBackgroundSection(job.Legs, 0))
	return sb.String()
}

func ClearAllBackground() {
	entries, err := os.ReadDir(filesystem.SessionsDir)
	if err != nil {
		return
	}
	jobMu.Lock()
	defer jobMu.Unlock()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := os.RemoveAll(filesystem.BackgroundDir(entry.Name())); err != nil {
			slog.Warn("ClearAllBackground",
				slog.String("session", entry.Name()),
				slog.String("error", err.Error()))
		}
	}
}
