package interactive

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type todoInput struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"active_form,omitempty"`
}

func registWriteTodo() {
	toolregister.Regist(toolregister.Def{
		Name:        "write_todo",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Shows the user a live task checklist; it records progress and runs nothing.
Call it before the first tool call when the answer takes more than one round of tool calls, or as soon as a running task grows past that; 分析 / 研究 / 調查 / 比較 / 評估 / 彙整 / 週報 / 盤前 always get one, and N fanned-out searches or subagents are N steps.
Smalltalk, or one lookup answered straight from one call → skip it.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"todos": map[string]any{
					"type":        "array",
					"description": "The full ordered list on every call (it replaces the previous one) with exactly one item in_progress; finishing a step marks it completed and the next in_progress in the same call. Tell the user before changing the remaining steps mid-run; a new objective after a completed list starts a fresh list.",
					"minItems":    1,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"content": map[string]any{
								"type":        "string",
								"description": "Imperative step title, e.g. 'Add lang toggle to nav'.",
							},
							"status": map[string]any{
								"type":        "string",
								"enum":        []string{agenttypes.TodoPending, agenttypes.TodoInProgress, agenttypes.TodoCompleted},
								"description": "pending, in_progress (the one step being worked on), or completed.",
							},
							"active_form": map[string]any{
								"type":        "string",
								"description": "Label shown while the step runs, e.g. 'Adding lang toggle'; omitted → content.",
							},
						},
						"required": []string{"content", "status"},
					},
				},
			},
			"required": []string{"todos"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Todos []todoInput `json:"todos"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			if len(params.Todos) == 0 {
				return "", utilsmsg.New(utilsmsg.Invalid, "todos", "must contain at least one item")
			}

			todos := make([]agenttypes.TodoItem, 0, len(params.Todos))
			var done, doing, pending int
			for i, in := range params.Todos {
				content := strings.TrimSpace(in.Content)
				if content == "" {
					return "", utilsmsg.New(utilsmsg.Invalid, "todos", "#%d has empty content", i+1)
				}
				status := strings.TrimSpace(in.Status)
				switch status {
				case agenttypes.TodoCompleted:
					done++
				case agenttypes.TodoInProgress:
					doing++
				case agenttypes.TodoPending, "":
					status = agenttypes.TodoPending
					pending++
				default:
					return "", utilsmsg.New(utilsmsg.Invalid, "todos", "#%d has invalid status %q (want pending / in_progress / completed)", i+1, in.Status)
				}
				todos = append(todos, agenttypes.TodoItem{
					Content:    content,
					Status:     status,
					ActiveForm: strings.TrimSpace(in.ActiveForm),
				})
			}
			if doing > 1 {
				return "", utilsmsg.New(utilsmsg.Invalid, "todos", "only one may be in_progress at a time, got %d", doing)
			}

			sessionID, taskHash := "", ""
			if e != nil {
				sessionID, taskHash = e.SessionID, e.PendingTask
			}
			if taskHash != "" {
				if err := WriteTodos(sessionID, taskHash, todos); err != nil {
					return "", utilsmsg.Trace("WriteTodos", "", err)
				}
			}

			result := fmt.Sprintf("checklist saved: %d step(s) — %d done, %d in progress, %d pending", len(todos), done, doing, pending)
			if doing == 0 && pending == 0 && !reportWritten(e) {
				result += "\nplan complete, no report written this task: long-form deliverable (analysis / research / comparison, past ~400 words, or a table plus commentary) → call write_result with every detail now, and reply with the key-point overview in that same message; short answer → reply directly"
			}
			return result, nil
		},
	})
}

func reportWritten(e *tooltypes.Executor) bool {
	for _, path := range e.EditedFiles() {
		name := filepath.Base(path)
		if strings.HasPrefix(name, "report-") && (strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".html")) {
			return true
		}
	}
	return false
}

func WriteTodos(sessionID, taskHash string, todos []agenttypes.TodoItem) error {
	if taskHash == "" {
		return nil
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()

	path := filesystem.PendingMetaPath(sessionID, taskHash)
	if !go_pkg_filesystem_reader.Exists(path) {
		return nil
	}
	meta, err := go_pkg_filesystem.ReadJSON[pendingMeta](path)
	if err != nil {
		meta = pendingMeta{}
	}
	meta.Todos = todos
	if writeErr := writePending(sessionID, taskHash, &meta); writeErr != nil {
		return utilsmsg.Trace("writePending", "", writeErr)
	}
	return nil
}

func LoadTodos(sessionID, taskHash string) []agenttypes.TodoItem {
	if taskHash == "" {
		return nil
	}
	meta, err := go_pkg_filesystem.ReadJSON[pendingMeta](filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil {
		return nil
	}
	return meta.Todos
}
