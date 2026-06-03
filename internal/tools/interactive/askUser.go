package interactive

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
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

type ToolResult struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Result string `json:"result"`
}

type pendingMeta struct {
	TaskHash    string             `json:"task_hash"`
	SessionID   string             `json:"session_id"`
	Questions   []runtime.Question `json:"questions"`
	ToolResults []ToolResult       `json:"tool_results,omitempty"`
}

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

func savePendingState(sessionID, taskHash string, state askState, questions []runtime.Question, toolResults []ToolResult) error {
	dir := filesystem.PendingDir(sessionID)
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return fmt.Errorf("CheckDir: %w", err)
	}

	var md strings.Builder
	md.WriteString("## Objective\n")
	md.WriteString(state.Objective)
	md.WriteString("\n\n## Completed\n")
	for _, s := range state.Completed {
		md.WriteString("- ")
		md.WriteString(s)
		md.WriteString("\n")
	}
	if len(toolResults) > 0 {
		md.WriteString("\n## Tool Results (this turn)\n")
		for _, tr := range toolResults {
			md.WriteString(fmt.Sprintf("- **%s**: %s\n", tr.Name, truncate(tr.Result, 200)))
		}
	}
	md.WriteString("\n## Next Steps\n")
	for _, s := range state.NextSteps {
		md.WriteString("- ")
		md.WriteString(s)
		md.WriteString("\n")
	}

	if err := go_pkg_filesystem.WriteFile(filesystem.PendingPath(sessionID, taskHash), md.String(), 0644); err != nil {
		return fmt.Errorf("WriteFile md: %w", err)
	}

	meta := pendingMeta{
		TaskHash:    taskHash,
		SessionID:   sessionID,
		Questions:   questions,
		ToolResults: toolResults,
	}
	if err := go_pkg_filesystem.WriteJSON(filesystem.PendingMetaPath(sessionID, taskHash), meta, true); err != nil {
		return fmt.Errorf("WriteFile json: %w", err)
	}
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

func LoadPendingMeta(sessionID, taskHash string) (pendingMeta, error) {
	return go_pkg_filesystem.ReadJSON[pendingMeta](filesystem.PendingMetaPath(sessionID, taskHash))
}

func CleanupPending(sessionID, taskHash string) {
	os.Remove(filesystem.PendingPath(sessionID, taskHash))
	os.Remove(filesystem.PendingMetaPath(sessionID, taskHash))
}

func ListPendingTasks(sessionID string) []string {
	dir := filesystem.PendingDir(sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var hashes []string
	seen := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		hash := strings.TrimSuffix(strings.TrimSuffix(name, ".md"), ".json")
		if !seen[hash] {
			seen[hash] = true
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

func PendingObjective(sessionID, taskHash string) string {
	content, err := go_pkg_filesystem.ReadText(filesystem.PendingPath(sessionID, taskHash))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "##") {
			return line
		}
	}
	return ""
}

func LoadResumeMessage(sessionID, taskHash string, answers []any) (string, error) {
	mdPath := filesystem.PendingPath(sessionID, taskHash)
	metaPath := filesystem.PendingMetaPath(sessionID, taskHash)

	stateContent, err := go_pkg_filesystem.ReadText(mdPath)
	if err != nil {
		return "", fmt.Errorf("ReadText %s: %w", mdPath, err)
	}

	meta, err := go_pkg_filesystem.ReadJSON[pendingMeta](metaPath)
	if err != nil {
		return "", fmt.Errorf("ReadJSON %s: %w", metaPath, err)
	}

	var msg strings.Builder
	msg.WriteString("[Resumed Task — ask_user response received]\n\n")
	msg.WriteString("## Previous Context\n")
	msg.WriteString(stateContent)

	if len(meta.ToolResults) > 0 {
		msg.WriteString("\n## Prior Tool Results\n")
		for _, tr := range meta.ToolResults {
			msg.WriteString(fmt.Sprintf("- **%s** (id=%s): %s\n", tr.Name, tr.ID, tr.Result))
		}
	}

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
	msg.WriteString("\nContinue executing the remaining steps based on the context above.")

	os.Remove(mdPath)
	os.Remove(metaPath)

	return msg.String(), nil
}

func SaveAndEnqueueAskUser(sessionID string, questions []runtime.Question, objective string, completed, nextSteps []string, toolResults []ToolResult) string {
	taskHash := go_pkg_utils.UUID()

	state := askState{Objective: objective, Completed: completed, NextSteps: nextSteps}
	if err := savePendingState(sessionID, taskHash, state, questions, toolResults); err != nil {
		slog.Warn("SaveAndEnqueueAskUser: savePendingState", slog.String("error", err.Error()))
	}

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
