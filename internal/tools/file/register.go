package file

import (
	"context"
	"encoding/json"
	"fmt"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	registReadFile()
	registListFiles()
	registGlobFiles()
	registSearchContent()
	registSearchHistory()
	registWriteFile()
	registPatchEdit()

	toolRegister.Regist(toolRegister.Def{
		Name:        "get_tool_error",
		Description: "Look up detailed information for a tool execution error by hash. Use when a tool returns 'no data: {hash}' to retrieve the full error context (tool_name, args, error message).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"hash": map[string]any{
					"type":        "string",
					"description": "Error identifier (8-char hex) from a tool response of the form 'no data: {hash}'",
				},
			},
			"required": []string{"hash"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Hash string `json:"hash"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			result := GetToolError(e.SessionID, params.Hash)
			if result == "" {
				return "not found", nil
			}
			return result, nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "remember_error",
		Description: "Record a tool error decision to persistent storage so future sessions can reference the root cause and resolution directly.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool_name": map[string]any{
					"type":        "string",
					"description": "Name of the tool that produced the error",
				},
				"keywords": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "Keywords for future lookup (tool name, error type, relevant parameter traits — be specific)",
				},
				"symptom": map[string]any{
					"type":        "string",
					"description": "Observed behavior (what the tool returned or what went wrong)",
				},
				"cause": map[string]any{
					"type":        "string",
					"description": "Root cause analysis (optional; fill in once confirmed)",
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Concrete action taken (e.g. retried with English keyword, fell back to search_web)",
				},
				"outcome": map[string]any{
					"type":        "string",
					"description": "Result of the action: resolved / failed / partial",
				},
			},
			"required": []string{"tool_name", "keywords", "symptom", "action"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ToolName string   `json:"tool_name"`
				Keywords []string `json:"keywords"`
				Symptom  string   `json:"symptom"`
				Cause    string   `json:"cause"`
				Action   string   `json:"action"`
				Outcome  string   `json:"outcome"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return SaveErrorMemory(e.SessionID, ErrorMemory{
				ToolName: params.ToolName,
				Keywords: params.Keywords,
				Symptom:  params.Symptom,
				Cause:    params.Cause,
				Action:   params.Action,
				Outcome:  params.Outcome,
			})
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "search_errors",
		Description: "Query past tool error records. Call first when a tool behaves unexpectedly — retrieves root cause and resolution from prior sessions. Searches across keywords, symptom, cause, and tool_name.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Search keyword (tool name, error symptom, or parameter trait — case-insensitive)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return. Default 4, max 16.",
					"default":     5,
				},
			},
			"required": []string{"keyword"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Keyword string `json:"keyword"`
				Limit   int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return SearchErrors(params.Keyword, params.Limit)
		},
	})
}
