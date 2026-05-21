package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const planningSystemPrompt = `You are a senior planner. Given a requirement, output ONE self-contained execution plan that another agent will follow step by step. You ONLY produce text; you have no tools and must not pretend to call any.

# Output structure (markdown, this exact order)

## 需求總結
- 需求是什麼: <verbatim requirement + any inferred scope>
- 要做到什麼: <completion criteria, scope, boundary>
- 結果要什麼: <output form, audience, acceptance>

## 前置條件
- <env / dependencies / permissions / data prerequisites>

## 步驟
1. **<Step title>**
   - 動作: <concrete operation, name the tool or command the executing agent should call>
   - 產出: <observable artifact / state>
   - 驗收: <how to prove this step is done>
   - 風險: <failure mode + mitigation>
2. ...

## 整體驗收
1. <end-to-end success criterion>
2. ...

## 風險與緩解
| 風險 | 機率 | 影響 | 緩解 |
|---|---|---|---|
| ... | 高/中/低 | 高/中/低 | ... |

## 回退方案
- <how to roll back / recover on failure>

# Rules

- Plan ONLY. Do not execute, fetch, or gather data — describe what the executing agent should do, do not do it yourself.
- Every step's "驗收" must be observable (file exists / value matches / state transition / response field present).
- Irreversible operations (delete, push, publish, migration, real trades) get their own step with dry-run or backup prerequisite.
- Sequential dependencies are implicit via numbering; parallelizable steps say "可與步驟 N 並行".
- Output ONLY the plan markdown. No preface, no postscript, no «here is the plan» line.`

func registGeneratePlan() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "generate_plan",
		AlwaysAllow: true,
		Concurrent:  false,
		Description: "Generate a detailed step-by-step execution plan from a requirement description. Use BEFORE starting a multi-step task where the user describes WHAT to achieve but not HOW — pass the requirement, receive a structured plan (需求總結 / 前置條件 / 步驟+驗收 / 整體驗收 / 風險表 / 回退方案), then execute it. This tool only generates the plan text; it never executes or gathers data.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"requirement": map[string]any{
					"type":        "string",
					"description": "The user's requirement or goal verbatim. Include any user-supplied scope hints, target names, deadlines. Do not summarize — pass full text.",
				},
				"context": map[string]any{
					"type":        "string",
					"description": "Optional extra context the planner should know: prior conversation findings, environment constraints (OS / language / framework), forbidden operations, available resources. Blank if none.",
					"default":     "",
				},
			},
			"required": []string{"requirement"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Requirement string `json:"requirement"`
				Context     string `json:"context,omitempty"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			requirement := strings.TrimSpace(params.Requirement)
			if requirement == "" {
				return "", fmt.Errorf("requirement is required")
			}

			dispatcher := agents.Dispatcher()
			registry := agents.Registry()
			if dispatcher == nil || len(registry.Registry) == 0 {
				return "", fmt.Errorf("planner host not initialized")
			}

			sessionID := ""
			if e != nil {
				sessionID = e.SessionID
			}
			agent := exec.SelectAgent(ctx, dispatcher, registry, "[plan] "+requirement, false, sessionID)
			if agent == nil {
				return "", fmt.Errorf("no agent available")
			}

			var userBuilder strings.Builder
			userBuilder.WriteString("Requirement:\n")
			userBuilder.WriteString(requirement)
			if extra := strings.TrimSpace(params.Context); extra != "" {
				userBuilder.WriteString("\n\nContext:\n")
				userBuilder.WriteString(extra)
			}

			messages := []agentTypes.Message{
				{Role: "system", Content: planningSystemPrompt},
				{Role: "user", Content: userBuilder.String()},
			}

			resp, err := agent.Send(ctx, messages, nil)
			if err != nil {
				return "", fmt.Errorf("agent.Send: %w", err)
			}
			if resp == nil || len(resp.Choices) == 0 {
				return "", fmt.Errorf("planner returned no choices")
			}

			content, ok := resp.Choices[0].Message.Content.(string)
			if !ok {
				return "", fmt.Errorf("planner response content is not a string")
			}
			plan := strings.TrimSpace(content)
			if plan == "" {
				return "", fmt.Errorf("planner returned empty plan")
			}
			return plan, nil
		},
	})
}
