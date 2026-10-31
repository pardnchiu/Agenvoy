package interactive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
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
	Args   string `json:"args,omitempty"`
	Result string `json:"result"`
}

type answeredQuestion struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type pendingMeta struct {
	TaskHash     string                `json:"task_hash"`
	SessionID    string                `json:"session_id"`
	DeliverTo    string                `json:"deliver_to,omitempty"`
	MessageID    string                `json:"message_id,omitempty"`
	Model        string                `json:"model,omitempty"`
	Reasoning    string                `json:"reasoning,omitempty"`
	Objective    string                `json:"objective,omitempty"`
	Completed    []string              `json:"completed,omitempty"`
	NextSteps    []string              `json:"next_steps,omitempty"`
	Questions    []runtime.Question    `json:"questions,omitempty"`
	Answer       []answeredQuestion    `json:"answer,omitempty"`
	ToolAttempts []ToolAttempt         `json:"tool_attempts,omitempty"`
	ToolResults  []ToolResult          `json:"tool_results,omitempty"`
	Todos        []agenttypes.TodoItem `json:"todos,omitempty"`
	Files        []string              `json:"files,omitempty"`
	Reply        string                `json:"reply,omitempty"`
	AllowAll     bool                  `json:"allow_all,omitempty"`
	Background   []BackgroundTask      `json:"background,omitempty"`
	WindowHash   string                `json:"window_hash,omitempty"`
}

var pendingMu sync.Mutex

func modelAvailable(name string) bool {
	registry := agents.ModelSet()
	agent, ok := registry.Registry[name]
	return ok && agent != nil
}

func registAskUser() {
	toolregister.Regist(toolregister.Def{
		Name:        "ask_user",
		Timeout:     toolregister.NoToolTimeout,
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  false,
		Description: `Puts one or more questions to the user and stops there — execution pauses, and a new turn resumes automatically once they answer.
Use when the target, scope, format or timing is not fixed by what they said, by an active Skill's SKILL.md, or by data a tool can fetch (git state, files), or when more than one tool would fit. A bare /<skill> command is fully specified by its SKILL.md: run it, never ask.
A credential is never asked for here → store_secret.`,
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
					"description": "Summarize current task state so execution can resume after the user responds.",
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
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Questions []askQuestion `json:"questions"`
				State     askState      `json:"state"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			if len(params.Questions) == 0 {
				return "", utilsmsg.New(utilsmsg.Invalid, "questions", "ask_user requires at least one question in 'questions'")
			}
			if strings.TrimSpace(params.State.Objective) == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "state.objective", "required for task resumption")
			}
			if len(params.State.NextSteps) == 0 {
				return "", utilsmsg.New(utilsmsg.Invalid, "state.next_steps", "required for task resumption")
			}

			questions := make([]runtime.Question, 0, len(params.Questions))
			for i, q := range params.Questions {
				q.Question = strings.TrimSpace(q.Question)
				if q.Question == "" {
					return "", utilsmsg.New(utilsmsg.Invalid, "questions", "#%d is empty", i+1)
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
				return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
			}
			return string(raw), nil
		},
	})
}

func clampPendingText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	head := limit * 3 / 4
	tail := limit - head
	elided := len(text) - head - tail
	return strings.ToValidUTF8(text[:head], "") +
		fmt.Sprintf("\n...[%d bytes elided]...\n", elided) +
		strings.ToValidUTF8(text[len(text)-tail:], "")
}

func readPending(path string) (pendingMeta, error) {
	raw, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return pendingMeta{}, utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.ReadText", path, err)
	}
	var meta pendingMeta
	if err := json.NewDecoder(strings.NewReader(raw)).Decode(&meta); err != nil {
		return pendingMeta{}, utilsmsg.Wrap(utilsmsg.Internal, "json.Decode", path, err)
	}
	return meta, nil
}

func writePending(sessionID, taskHash string, meta *pendingMeta) error {
	dir := filesystem.PendingDir(sessionID)
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", dir, err)
	}

	meta.TaskHash = taskHash
	meta.SessionID = sessionID
	for i := range meta.ToolResults {
		meta.ToolResults[i].Args = clampPendingText(meta.ToolResults[i].Args, configs.MAX_PENDING_ARGS_BYTES)
		meta.ToolResults[i].Result = clampPendingText(meta.ToolResults[i].Result, configs.MAX_PENDING_RESULT_BYTES)
	}

	if err := go_pkg_filesystem.WriteJSON(filesystem.PendingMetaPath(sessionID, taskHash), meta, false); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteJSON", "", err)
	}
	return nil
}

func FinalizePending(sessionID, taskHash, reply string) {
	if taskHash == "" {
		return
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return
	}
	meta.Reply = reply
	if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
		slog.Debug("FinalizePending",
			slog.String("session", sessionID),
			slog.String("error", writeErr.Error()))
	}
}

func RenamePending(sessionID, oldHash, newHash string) error {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, oldHash))
	if err != nil {
		return utilsmsg.Trace("readPending", oldHash, err)
	}
	if err := writePending(sessionID, newHash, &meta); err != nil {
		return utilsmsg.Trace("writePending", newHash, err)
	}
	if err := os.Remove(filesystem.PendingMetaPath(sessionID, oldHash)); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os.Remove", oldHash, err)
	}
	return nil
}

func DeletePending(sessionID, taskHash string) {
	if taskHash == "" {
		return
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()
	os.Remove(filesystem.PendingMetaPath(sessionID, taskHash))
}

func CompactPending(sessionID, taskHash string) {
	if taskHash == "" {
		return
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return
	}

	for i := range meta.ToolResults {
		meta.ToolResults[i].Args = minify(meta.ToolResults[i].Args)
		meta.ToolResults[i].Result = minify(meta.ToolResults[i].Result)
	}

	if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
		slog.Debug("CompactPending", slog.String("session", sessionID), slog.String("error", writeErr.Error()))
	}
}

func minify(str string) string {
	trimmed := strings.TrimSpace(str)
	if trimmed == "" {
		return ""
	}
	var buf bytes.Buffer
	if json.Compact(&buf, []byte(trimmed)) == nil {
		return buf.String()
	}
	return strings.Join(strings.Fields(trimmed), " ")
}

func CleanupPending(sessionID, taskHash string) {
	if taskHash == "" {
		return
	}
	CompactPending(sessionID, taskHash)

	src := filesystem.PendingMetaPath(sessionID, taskHash)
	if !go_pkg_filesystem_reader.Exists(src) {
		return
	}

	raw, err := go_pkg_filesystem.ReadText(src)
	if err != nil {
		slog.Warn("CleanupPending ReadText",
			slog.String("file", src),
			slog.String("error", err.Error()))
		return
	}

	var row historystore.ActionRecord
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		slog.Warn("CleanupPending Unmarshal",
			slog.String("file", src),
			slog.String("error", err.Error()))
		return
	}
	row.TaskHash = taskHash
	row.EndAt = time.Now()

	if err := historystore.WriteAction(context.Background(), sessionID, row); err != nil {
		slog.Warn("historystore.WriteAction",
			slog.String("session", sessionID),
			slog.String("task", taskHash),
			slog.String("error", err.Error()))
		return
	}

	os.Remove(src)
}

var pendingSkipTool = map[string]bool{"write_todo": true}

func CreateExecPending(sessionID, taskHash, objective, messageID, model, reasoning string, allowAll bool) string {
	if taskHash == "" {
		taskHash = go_pkg_utils.UUID()
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()

	sessionModel, sessionReasoning := configbot.GetModel(sessionID)
	if strings.TrimSpace(model) == "" {
		model = sessionModel
	}
	if strings.TrimSpace(reasoning) == "" {
		reasoning = sessionReasoning
	}
	if err := writePending(sessionID, taskHash, &pendingMeta{Objective: objective, MessageID: messageID, Model: model, Reasoning: reasoning, AllowAll: allowAll}); err != nil {
		slog.Warn("CreateExecPending", slog.String("session", sessionID), slog.String("error", err.Error()))
	}
	return taskHash
}

func LoadPendingAllowAll(sessionID, taskHash string) bool {
	if taskHash == "" {
		return false
	}
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return false
	}
	return meta.AllowAll
}

func LoadPendingMessageID(sessionID, taskHash string) string {
	if taskHash == "" {
		return ""
	}
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return ""
	}
	return meta.MessageID
}

func RecordToolAttempt(sessionID, taskHash string, attempt ToolAttempt) {
	if taskHash == "" {
		return
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return
	}
	meta.ToolAttempts = []ToolAttempt{attempt}
	if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
		slog.Debug("RecordToolAttempt", slog.String("session", sessionID), slog.String("error", writeErr.Error()))
	}
}

func AppendToolResult(sessionID, taskHash string, result ToolResult, files []string) {
	if taskHash == "" {
		return
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()

	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return
	}
	meta.Files = mergeFiles(meta.Files, files)
	if pendingSkipTool[result.Name] {
		meta.ToolAttempts = nil
		if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
			slog.Debug("AppendToolResult", slog.String("session", sessionID), slog.String("error", writeErr.Error()))
		}
		return
	}
	for _, existing := range meta.ToolResults {
		if existing.ID == result.ID {
			if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
				slog.Debug("AppendToolResult", slog.String("session", sessionID), slog.String("error", writeErr.Error()))
			}
			return
		}
	}
	meta.ToolResults = append(meta.ToolResults, result)
	meta.ToolAttempts = nil
	if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
		slog.Debug("AppendToolResult", slog.String("session", sessionID), slog.String("error", writeErr.Error()))
	}
}

func listPendingTasks(sessionID string) []string {
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

func mergeFiles(existing, added []string) []string {
	out := slices.Clone(existing)
	for _, one := range added {
		if one != "" && !slices.Contains(out, one) {
			out = append(out, one)
		}
	}
	return out
}

func LoadPendingFiles(sessionID, taskHash string) []string {
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return nil
	}
	return meta.Files
}

type PendingInfo struct {
	TaskHash     string
	Objective    string
	HasQuestions bool
	UpdatedAt    time.Time
}

func LoadPendingInfo(sessionID, taskHash string) (PendingInfo, bool) {
	path := filesystem.PendingMetaPath(sessionID, taskHash)
	meta, err := readPending(path)
	if err != nil {
		return PendingInfo{}, false
	}

	info := PendingInfo{
		TaskHash:     taskHash,
		Objective:    meta.Objective,
		HasQuestions: len(meta.Questions) > 0,
	}
	// * os.Stat retained: go-pkg exposes no ModTime accessor
	if stat, err := os.Stat(path); err == nil {
		info.UpdatedAt = stat.ModTime()
	}
	return info, true
}

func LoadPendingQuestions(sessionID, taskHash string) ([]runtime.Question, error) {
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return nil, err
	}
	return meta.Questions, nil
}

func buildResumeAnswer(answer any) string {
	switch v := answer.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		raw, _ := json.Marshal(v)
		return string(raw)
	}
}

func LoadResumeMessage(sessionID, taskHash string, answers []any) (full string, history string, err error) {
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return "", "", utilsmsg.Trace("readPending", "", err)
	}

	if meta.Model != "" {
		model := meta.Model
		if model != configbot.DefaultModel && !modelAvailable(model) {
			slog.Info("resume: recorded model is gone, falling back to auto",
				slog.String("session", sessionID),
				slog.String("model", meta.Model))
			model = configbot.DefaultModel
		}
		configbot.SetModel(sessionID, model, meta.Reasoning)
	}

	var msg strings.Builder

	finished, running := backgroundToReport(meta.Background)
	if len(meta.Questions) > 0 {
		msg.WriteString("[Resumed Task — ask_user response received]\n\n")
	} else if len(finished) > 0 {
		msg.WriteString("[Resumed Task — background task finished]\n\n")
	} else {
		msg.WriteString("[Resumed Task — interrupted execution recovered]\n\n")
	}

	msg.WriteString("## Objective\n")
	msg.WriteString(meta.Objective)
	msg.WriteString("\n")

	if len(meta.Todos) > 0 {
		msg.WriteString("\n## Task Checklist\n")
		msg.WriteString("Your last recorded plan state. Keep it current via `write_todo` as you resume.\n")
		for _, td := range meta.Todos {
			mark := "[ ]"
			switch td.Status {
			case agenttypes.TodoCompleted:
				mark = "[x]"
			case agenttypes.TodoInProgress:
				mark = "[~]"
			}
			msg.WriteString(fmt.Sprintf("- %s %s\n", mark, td.Content))
		}
	}

	if len(meta.Completed) > 0 {
		msg.WriteString("\n## Completed Steps\n")
		for _, s := range meta.Completed {
			msg.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	completedIDs := make(map[string]bool, len(meta.ToolResults))
	for _, tr := range meta.ToolResults {
		completedIDs[tr.ID] = true
	}

	if len(meta.ToolResults) > 0 {
		msg.WriteString("\n## Completed Tool Results\n")
		msg.WriteString("Actual output from each tool call completed before this task was interrupted — this is the ground truth referenced above, use it directly instead of re-deriving or guessing. A result or args containing `...[N bytes elided]...` is truncated: when the elided part matters, call the same tool with the same args again instead of filling the gap yourself.\n")
		for _, tr := range meta.ToolResults {
			msg.WriteString(fmt.Sprintf("\n### %s (id=%s)\n", tr.Name, tr.ID))
			if tr.Args != "" {
				msg.WriteString(fmt.Sprintf("args=%s\n", tr.Args))
			}
			msg.WriteString(tr.Result)
			msg.WriteString("\n")
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

	if len(meta.Answer) > 0 {
		msg.WriteString("\n## Answers\n")
		for i, a := range meta.Answer {
			msg.WriteString(fmt.Sprintf("%d. Q: %s\n   A: %s\n", i+1, a.Question, a.Answer))
		}
	}

	var sb strings.Builder
	if len(meta.Questions) > 0 {
		sb.WriteString("[Resumed Task — ask_user response received]\n\n")
		if obj := strings.TrimSpace(meta.Objective); obj != "" {
			sb.WriteString("## User Input\n")
			sb.WriteString(obj)
			sb.WriteString("\n\n")
		}
		sb.WriteString("## Current Answers\n")
		for i, q := range meta.Questions {
			sb.WriteString(fmt.Sprintf("%d. Q: %s\n", i+1, q.Question))
			if i < len(answers) {
				sb.WriteString(fmt.Sprintf("   A: %s\n", buildResumeAnswer(answers[i])))
			}
		}
		msg.WriteString("\n## Current Answers\n")
		for i, q := range meta.Questions {
			msg.WriteString(fmt.Sprintf("%d. Q: %s\n", i+1, q.Question))
			if i < len(answers) {
				msg.WriteString(fmt.Sprintf("   A: %s\n", buildResumeAnswer(answers[i])))
			}
		}
	}

	if len(meta.NextSteps) > 0 {
		msg.WriteString("\n## Remaining Steps\n")
		for _, s := range meta.NextSteps {
			msg.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	if len(finished) > 0 {
		section := formatBackgroundSection(finished, running)
		msg.WriteString(section)
		if sb.Len() == 0 {
			sb.WriteString("[Resumed Task — background task finished]\n")
			sb.WriteString(section)
		}
		markBackgroundReported(sessionID, taskHash, finished)
	}

	msg.WriteString("\nContinue from where this task was interrupted. Use the completed tool results as ground truth.")

	if len(meta.Questions) > 0 {
		pendingMu.Lock()
		for i, q := range meta.Questions {
			answer := ""
			if i < len(answers) {
				answer = buildResumeAnswer(answers[i])
			}
			meta.Answer = append(meta.Answer, answeredQuestion{Question: q.Question, Answer: answer})
		}
		meta.Questions = nil
		if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
			slog.Debug("LoadResumeMessage: move answered questions",
				slog.String("session", sessionID),
				slog.String("task_hash", taskHash),
				slog.String("error", writeErr.Error()))
		}
		pendingMu.Unlock()
	}

	runtime.DropTask(sessionID, taskHash)
	endEvent := agenttypes.Event{
		Type:           agenttypes.EventPendingEnd,
		Text:           taskHash,
		TaskHash:       taskHash,
		PendingSession: sessionID,
	}
	pubsub.Pub(sessionID, endEvent)
	if meta.DeliverTo != "" && meta.DeliverTo != sessionID {
		pubsub.Pub(meta.DeliverTo, endEvent)
	}

	return msg.String(), sb.String(), nil
}

func deliverFor(ctx context.Context, sessionID string) string {
	if id := agenttypes.DeliverToFrom(ctx); id != "" {
		return id
	}
	return sessionID
}

func originFor(ctx context.Context, sessionID string) string {
	if origin := agenttypes.OriginFrom(ctx); origin != "" {
		return origin
	}
	return runtime.OriginOf(sessionID)
}

func SaveAndEnqueueAskUser(sessionID, origin, deliverTo string, questions []runtime.Question, objective string, completed, nextSteps []string, toolResults []ToolResult, existingTaskHash string, files []string) string {
	taskHash := existingTaskHash
	if taskHash == "" {
		taskHash = go_pkg_utils.UUID()
	}

	pendingMu.Lock()
	var allResults []ToolResult
	var messageID, model, reasoning string
	var allowAll bool
	allFiles := files
	if existing, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash)); err == nil {
		allowAll = existing.AllowAll
		messageID = existing.MessageID
		model = existing.Model
		reasoning = existing.Reasoning
		allFiles = mergeFiles(existing.Files, files)
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
		model, reasoning = configbot.GetModel(sessionID)
	}
	if err := writePending(sessionID, taskHash, &pendingMeta{
		MessageID:   messageID,
		DeliverTo:   deliverTo,
		Model:       model,
		Reasoning:   reasoning,
		Objective:   objective,
		Completed:   completed,
		NextSteps:   nextSteps,
		Questions:   questions,
		Files:       allFiles,
		ToolResults: allResults,
		AllowAll:    allowAll,
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
			if errors.Is(reply.Error, runtime.ErrUserCanceled) {
				CleanupPending(sessionID, taskHash)
				runtime.NotifyCanceled(sessionID, taskHash, reply.Error.Error())
			}
			return
		}
		runtime.TriggerResume(sessionID, taskHash, reply.Answers)
	}

	if _, err := runtime.AskUser(runtime.Request{
		Kind:      runtime.KindAskUser,
		SessionID: sessionID,
		TaskHash:  taskHash,
		Origin:    origin,
		DeliverTo: deliverTo,
		ToolName:  "ask_user",
		AskUser:   &runtime.UserPayload{Questions: questions},
	}, onResolve); err != nil {
		slog.Warn("SaveAndEnqueueAskUser: AskAsync", slog.String("error", err.Error()))
	}

	return taskHash
}

func AskPrompt(ctx context.Context, sessionID string, questions []runtime.Question) ([]any, error) {
	origin := originFor(ctx, sessionID)
	if !runtime.HasListener(ctx, origin) {
		return nil, utilsmsg.New(utilsmsg.Unavailable, "ask_user", "requires an interactive channel (TUI / Telegram / Discord)")
	}

	reply, err := runtime.Ask(ctx, runtime.Request{
		Kind:      runtime.KindAskUser,
		SessionID: sessionID,
		Origin:    origin,
		DeliverTo: deliverFor(ctx, sessionID),
		ToolName:  "ask_user",
		AskUser:   &runtime.UserPayload{Questions: questions},
		Inline:    true,
	})
	if err != nil {
		return nil, utilsmsg.Trace("runtime.Ask", "", err)
	}
	if reply.Error != nil {
		return nil, utilsmsg.Trace("runtime.Ask", "", reply.Error)
	}
	return reply.Answers, nil
}
