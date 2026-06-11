package interactive

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

type askQuestion struct {
	Question    string   `json:"question"`
	Detail      string   `json:"detail,omitempty"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
}

type askState struct {
	Objective string   `json:"objective"`
	Completed []string `json:"completed"`
	NextSteps []string `json:"next_steps"`
}

type ToolAttempt struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	Args string `json:"args"`
}

type ToolResult struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Result string `json:"result"`
}

type pendingMeta struct {
	TaskHash     string             `json:"task_hash"`
	SessionID    string             `json:"session_id"`
	Objective    string             `json:"objective,omitempty"`
	Completed    []string           `json:"completed,omitempty"`
	NextSteps    []string           `json:"next_steps,omitempty"`
	Questions    []runtime.Question `json:"questions,omitempty"`
	ToolAttempts []ToolAttempt      `json:"tool_attempts,omitempty"`
	ToolResults  []ToolResult       `json:"tool_results,omitempty"`
	Reply        string             `json:"reply,omitempty"`
}

var pendingMu sync.Mutex

func registAskUser() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "ask_user",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Description: "Ask the user one or more questions. Execution pauses until the user responds; a new turn resumes automatically with full context.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"questions": map[string]any{
					"type":        "array",
					"description": "Questions to ask in order. Each item is either a free-text prompt (no options) or a choice prompt (with options).",
					"minItems":    1,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"question": map[string]any{
								"type":        "string",
								"description": "Short prompt shown as the popup title (the actual question, e.g. 'Confirm?').",
							},
							"detail": map[string]any{
								"type":        "string",
								"description": "Optional multi-line context / details rendered as popup subtitle in hint style above the input. Use this for the supporting info (list of detected items, current values, etc.) — keep `question` short.",
							},
							"options": map[string]any{
								"type":        "array",
								"description": "Fixed choices. Omit or empty for free-text input.",
								"items":       map[string]any{"type": "string"},
							},
							"multi_select": map[string]any{
								"type":        "boolean",
								"description": "When options is non-empty: true = multi-select (comma-separated indices), false = single-select (arrow keys). Ignored for free-text.",
							},
							"secret": map[string]any{
								"type":        "boolean",
								"description": "Free-text only: true masks input and excludes the answer from logs. Ignored when options is set.",
							},
						},
						"required": []string{"question"},
					},
				},
				"state": map[string]any{
					"type":        "object",
					"description": "Context snapshot for task resumption. Summarize current task state so execution can resume after user responds.",
					"properties": map[string]any{
						"objective": map[string]any{
							"type":        "string",
							"description": "Original user request (verbatim or paraphrased).",
						},
						"completed": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Steps already completed with key results.",
						},
						"next_steps": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "What to do after receiving the user's answers.",
						},
					},
					"required": []string{"objective", "completed", "next_steps"},
				},
			},
			"required": []string{"questions", "state"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Questions []askQuestion `json:"questions"`
				State     askState      `json:"state"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json Unmarshal: %w", err)
			}
			if len(params.Questions) == 0 {
				return "", fmt.Errorf("ask_user requires at least one question in 'questions'")
			}
			if strings.TrimSpace(params.State.Objective) == "" {
				return "", fmt.Errorf("state.objective is required for task resumption")
			}
			if len(params.State.NextSteps) == 0 {
				return "", fmt.Errorf("state.next_steps is required for task resumption")
			}

			questions := make([]runtime.Question, 0, len(params.Questions))
			for i, q := range params.Questions {
				q.Question = strings.TrimSpace(q.Question)
				if q.Question == "" {
					return "", fmt.Errorf("question #%d is empty", i+1)
				}
				questions = append(questions, runtime.Question{
					Question:    q.Question,
					Detail:      strings.TrimSpace(q.Detail),
					Options:     q.Options,
					MultiSelect: q.MultiSelect,
					Secret:      q.Secret,
				})
			}

			answers, err := AskPrompt(ctx, e.SessionID, questions)
			if err != nil {
				return "", err
			}

			raw, err := json.Marshal(map[string]any{"answers": answers})
			if err != nil {
				return "", fmt.Errorf("json Marshal: %w", err)
			}
			return string(raw), nil
		},
	})
}

func writePending(sessionID, taskHash string, meta *pendingMeta) error {
	dir := filesystem.PendingDir(sessionID)
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return fmt.Errorf("CheckDir: %w", err)
	}

	meta.TaskHash = taskHash
	meta.SessionID = sessionID

	if err := go_pkg_filesystem.WriteJSON(filesystem.PendingMetaPath(sessionID, taskHash), meta, true); err != nil {
		return fmt.Errorf("WriteFile json: %w", err)
	}
	return nil
}

func FinalizePending(sessionID, taskHash, reply string) {
	if taskHash == "" {
		return
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := go_pkg_filesystem.ReadJSON[pendingMeta](filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return
	}
	meta.Reply = reply
	if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
		slog.Warn("FinalizePending",
			slog.String("session", sessionID),
			slog.String("error", writeErr.Error()))
	}
}

func CleanupPending(sessionID, taskHash string) {
	if taskHash == "" {
		return
	}
	src := filesystem.PendingMetaPath(sessionID, taskHash)
	if !go_pkg_filesystem_reader.Exists(src) {
		return
	}

	histDir := filesystem.TaskHistoryDir(sessionID)
	if err := go_pkg_filesystem.CheckDir(histDir, true); err != nil {
		slog.Warn("CleanupPending CheckDir",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
		os.Remove(src)
		return
	}

	ts := time.Now().Format("2006-01-02-15-04")
	dst := filepath.Join(histDir, fmt.Sprintf("%s-%s.json", ts, taskHash))
	if err := os.Rename(src, dst); err != nil {
		slog.Warn("CleanupPending rename",
			slog.String("src", src),
			slog.String("dst", dst),
			slog.String("error", err.Error()))
		os.Remove(src)
	}
}

func CreateExecPending(sessionID, objective string) string {
	taskHash := go_pkg_utils.UUID()
	pendingMu.Lock()
	defer pendingMu.Unlock()

	cleanStaleProgress(sessionID)

	if err := writePending(sessionID, taskHash, &pendingMeta{Objective: objective}); err != nil {
		slog.Warn("CreateExecPending", slog.String("session", sessionID), slog.String("error", err.Error()))
	}
	return taskHash
}

func cleanStaleProgress(sessionID string) {
	for _, hash := range ListPendingTasks(sessionID) {
		meta, err := go_pkg_filesystem.ReadJSON[pendingMeta](filesystem.PendingMetaPath(sessionID, hash))
		if err != nil {
			continue
		}
		if len(meta.Questions) == 0 {
			os.Remove(filesystem.PendingMetaPath(sessionID, hash))
		}
	}
}

func RecordToolAttempt(sessionID, taskHash string, attempt ToolAttempt) {
	if taskHash == "" {
		return
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := go_pkg_filesystem.ReadJSON[pendingMeta](filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return
	}
	meta.ToolAttempts = []ToolAttempt{attempt}
	if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
		slog.Warn("RecordToolAttempt", slog.String("session", sessionID), slog.String("error", writeErr.Error()))
	}
}

func AppendToolResult(sessionID, taskHash string, result ToolResult) {
	if taskHash == "" {
		return
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := go_pkg_filesystem.ReadJSON[pendingMeta](filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return
	}
	for _, existing := range meta.ToolResults {
		if existing.ID == result.ID {
			return
		}
	}
	meta.ToolResults = append(meta.ToolResults, result)
	meta.ToolAttempts = nil
	if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
		slog.Warn("AppendToolResult", slog.String("session", sessionID), slog.String("error", writeErr.Error()))
	}
}

func ListPendingTasks(sessionID string) []string {
	files, err := go_pkg_filesystem_reader.ListFiles(filesystem.PendingDir(sessionID))
	if err != nil {
		return nil
	}
	var hashes []string
	for _, f := range files {
		if !strings.HasSuffix(f.Name, ".json") {
			continue
		}
		hashes = append(hashes, strings.TrimSuffix(f.Name, ".json"))
	}
	return hashes
}

type PendingInfo struct {
	TaskHash     string
	Objective    string
	HasQuestions bool
}

func LoadPendingInfo(sessionID, taskHash string) (PendingInfo, bool) {
	meta, err := go_pkg_filesystem.ReadJSON[pendingMeta](filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return PendingInfo{}, false
	}
	return PendingInfo{
		TaskHash:     taskHash,
		Objective:    meta.Objective,
		HasQuestions: len(meta.Questions) > 0,
	}, true
}

func LoadPendingQuestions(sessionID, taskHash string) ([]runtime.Question, error) {
	meta, err := go_pkg_filesystem.ReadJSON[pendingMeta](filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return nil, err
	}
	return meta.Questions, nil
}

func LoadResumeMessage(sessionID, taskHash string, answers []any) (string, error) {
	meta, err := go_pkg_filesystem.ReadJSON[pendingMeta](filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return "", fmt.Errorf("ReadJSON: %w", err)
	}

	var msg strings.Builder

	if len(meta.Questions) > 0 {
		msg.WriteString("[Resumed Task — ask_user response received]\n\n")
	} else {
		msg.WriteString("[Resumed Task — interrupted execution recovered]\n\n")
	}

	msg.WriteString("## Objective\n")
	msg.WriteString(meta.Objective)
	msg.WriteString("\n")

	if len(meta.Completed) > 0 {
		msg.WriteString("\n## Completed Steps\n")
		for _, s := range meta.Completed {
			msg.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	completedIDs := make(map[string]bool, len(meta.ToolResults))
	if len(meta.ToolResults) > 0 {
		msg.WriteString("\n## Completed Tool Results\n")
		msg.WriteString("These tools have already been executed. Results are final — do not re-execute.\n")
		for _, tr := range meta.ToolResults {
			completedIDs[tr.ID] = true
			msg.WriteString(fmt.Sprintf("- **%s** (id=%s): %s\n", tr.Name, tr.ID, tr.Result))
		}
	}

	var interrupted []ToolAttempt
	for _, ta := range meta.ToolAttempts {
		if !completedIDs[ta.ID] {
			interrupted = append(interrupted, ta)
		}
	}
	if len(interrupted) > 0 {
		msg.WriteString("\n## Interrupted Tool Calls\n")
		msg.WriteString("These tools were started but did not complete. Decide whether to retry based on the objective.\n")
		for _, ta := range interrupted {
			msg.WriteString(fmt.Sprintf("- **%s** (id=%s): args=%s\n", ta.Name, ta.ID, ta.Args))
		}
	}

	if len(meta.Questions) > 0 {
		msg.WriteString("\n## User Answers\n")
		for i, q := range meta.Questions {
			msg.WriteString(fmt.Sprintf("%d. Q: %s\n", i+1, q.Question))
			if i < len(answers) {
				switch v := answers[i].(type) {
				case string:
					msg.WriteString(fmt.Sprintf("   A: %s\n", v))
				case []any:
					parts := make([]string, 0, len(v))
					for _, item := range v {
						if s, ok := item.(string); ok {
							parts = append(parts, s)
						}
					}
					msg.WriteString(fmt.Sprintf("   A: %s\n", strings.Join(parts, ", ")))
				default:
					raw, _ := json.Marshal(v)
					msg.WriteString(fmt.Sprintf("   A: %s\n", string(raw)))
				}
			}
		}
	}

	if len(meta.NextSteps) > 0 {
		msg.WriteString("\n## Remaining Steps\n")
		for _, s := range meta.NextSteps {
			msg.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	msg.WriteString("\nContinue from where this task was interrupted. Use the completed tool results as ground truth.")

	return msg.String(), nil
}

func SaveAndEnqueueAskUser(sessionID string, questions []runtime.Question, objective string, completed, nextSteps []string, toolResults []ToolResult, existingTaskHash string) string {
	taskHash := existingTaskHash
	if taskHash == "" {
		taskHash = go_pkg_utils.UUID()
	}

	pendingMu.Lock()
	var allResults []ToolResult
	if existing, err := go_pkg_filesystem.ReadJSON[pendingMeta](filesystem.PendingMetaPath(sessionID, taskHash)); err == nil {
		seen := make(map[string]bool, len(toolResults))
		for _, r := range toolResults {
			if r.ID != "" {
				seen[r.ID] = true
			}
		}
		allResults = append(allResults, toolResults...)
		for _, r := range existing.ToolResults {
			if r.ID != "" && !seen[r.ID] {
				allResults = append(allResults, r)
			}
		}
	} else {
		allResults = toolResults
	}
	if err := writePending(sessionID, taskHash, &pendingMeta{
		Objective:   objective,
		Completed:   completed,
		NextSteps:   nextSteps,
		Questions:   questions,
		ToolResults: allResults,
	}); err != nil {
		slog.Warn("SaveAndEnqueueAskUser: writePending", slog.String("error", err.Error()))
	}
	pendingMu.Unlock()

	onResolve := func(reply runtime.Reply) {
		if reply.Error != nil {
			slog.Warn("ask_user async resolve error",
				slog.String("session", sessionID),
				slog.String("task_hash", taskHash),
				slog.String("error", reply.Error.Error()))
			CleanupPending(sessionID, taskHash)
			return
		}
		runtime.TriggerResume(sessionID, taskHash, reply.Answers)
	}

	if _, err := runtime.AskUser(runtime.Request{
		Kind:      runtime.KindAskUser,
		SessionID: sessionID,
		ToolName:  "ask_user",
		AskUser:   &runtime.UserPayload{Questions: questions},
	}, onResolve); err != nil {
		slog.Warn("SaveAndEnqueueAskUser: AskAsync", slog.String("error", err.Error()))
	}

	return taskHash
}

func AskPrompt(ctx context.Context, sessionID string, questions []runtime.Question) ([]any, error) {
	if !runtime.HasListener(sessionID) {
		return nil, fmt.Errorf("ask_user requires an interactive channel (TUI / Telegram / Discord)")
	}

	reply, err := runtime.Ask(ctx, runtime.Request{
		Kind:      runtime.KindAskUser,
		SessionID: sessionID,
		ToolName:  "ask_user",
		AskUser:   &runtime.UserPayload{Questions: questions},
	})
	if err != nil {
		return nil, fmt.Errorf("runtime Ask: %w", err)
	}
	if reply.Error != nil {
		return nil, fmt.Errorf("runtime Ask: %w", reply.Error)
	}
	return reply.Answers, nil
}
