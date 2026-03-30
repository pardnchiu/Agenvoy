package externalAgent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registVerifyWithExternalAgent() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "verify_with_external_agent",
		Description: `將當前結果送交所有可用外部 agent（codex / copilot / claude）並行審查，回傳各 agent 的獨立回饋供主 agent 參考修正。外部 agent 在獨立環境執行，無法使用本專案 tool。用戶明確要求驗證、審查、交叉確認、second opinion 時才呼叫。無可用外部 agent 時回傳降級訊息，不阻斷主流程。`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{
					"type":        "string",
					"description": "原始問題或任務描述",
				},
				"result": map[string]any{
					"type":        "string",
					"description": "待驗證的結果內容",
				},
			},
			"required": []string{"input", "result"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Input  string `json:"input"`
				Result string `json:"result"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			if len(GetAgents()) == 0 {
				return `外部驗證已忽略：未宣告任何外部 agent。若需外部驗證，請在環境變數設定 EXTERNAL_CODEX=true / EXTERNAL_COPILOT=true / EXTERNAL_CLAUDE=true，並安裝對應 CLI。`, nil
			}

			agents, errors := checkUsefulAgents()
			if len(agents) == 0 {
				var sb strings.Builder
				sb.WriteString("無可用外部 agent，忽略外部驗證：\n")
				for agent, err := range errors {
					sb.WriteString(fmt.Sprintf("- %s 不可用：%s\n", agent, err.Error()))
				}
				return sb.String(), nil
			}

			prompt := buildCheckPrompt(params.Input, params.Result)
			results := runParallel(ctx, agents, prompt)
			output := formatFeedback(results)
			if len(errors) > 0 {
				var note strings.Builder
				note.WriteString("以下 agent 不可用（已忽略）：\n")
				for agent, err := range errors {
					note.WriteString(fmt.Sprintf("- %s：%s\n", agent, err.Error()))
				}
				output = note.String() + "\n" + output
			}
			return output, nil
		},
	})
}

func checkUsefulAgents() ([]string, map[string]error) {
	var agents []string
	errors := make(map[string]error)
	for _, a := range GetAgents() {
		if err := checkCLI(a); err != nil {
			errors[a] = err
		} else {
			agents = append(agents, a)
		}
	}
	return agents, errors
}

func buildCheckPrompt(input, result string) string {
	return fmt.Sprintf(
		`請審查以下任務的執行結果，指出具體問題並給出改進方向。若結果已完整正確，請明確回應「通過」。

## 任務輸入
%s

## 當前結果
%s

請直接指出問題（如有），或確認通過。`,
		input, result,
	)
}

func runParallel(ctx context.Context, agents []string, prompt string) []agentResult {
	ch := make(chan agentResult, len(agents))
	for _, a := range agents {
		go func(agent string) {
			out, err := runOne(ctx, agent, prompt)
			ch <- agentResult{Agent: agent, Output: out, Err: err}
		}(a)
	}

	results := make([]agentResult, 0, len(agents))
	for range agents {
		results = append(results, <-ch)
	}
	return results
}

func formatFeedback(results []agentResult) string {
	var sb strings.Builder
	sb.WriteString("外部驗證回饋結果\n\n")
	for _, r := range results {
		if r.Err != nil {
			sb.WriteString(fmt.Sprintf("[%s] ❌ %s\n\n", r.Agent, r.Err.Error()))
		} else {
			preview := r.Output
			if len(preview) > 600 {
				preview = preview[:600] + "…"
			}
			sb.WriteString(fmt.Sprintf("[%s]\n%s\n\n", r.Agent, preview))
		}
	}
	return sb.String()
}
