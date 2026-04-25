package externalAgent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/external"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registCrossReviewWithExternalAgents() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "cross_review_with_external_agents",
		ReadOnly: true,
		Description: `將**已產出的真實結果**送交所有可用外部 agent（codex／copilot／claude／gemini）並行交叉驗證，取得隔離環境下的獨立審查意見與改進建議，供主 agent 據此修正。

呼叫條件（三者皆須滿足）：
1. 已有具體完成的產出（程式碼／文件／決策／分析等），非佔位或假設內容
2. 使用者明確要求驗證／交叉確認／second opinion，或任務風險高需外部把關
3. 目的是檢查本 agent 輸出是否正確／完備，不是做其他事

無可用 agent 時回傳降級訊息，不阻斷主流程。`,
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

			if len(external.Agents()) == 0 {
				return `外部驗證已忽略：未宣告任何外部 agent。若需外部驗證，請在環境變數設定 EXTERNAL_CODEX=true / EXTERNAL_COPILOT=true / EXTERNAL_CLAUDE=true / EXTERNAL_GEMINI=true，並安裝對應 CLI。`, nil
			}

			agents, errors := external.CheckAgents()
			if len(agents) == 0 {
				var sb strings.Builder
				sb.WriteString("無可用外部 agent，忽略外部驗證：\n")
				for agent, err := range errors {
					sb.WriteString(fmt.Sprintf("- %s 不可用：%s\n", agent, err.Error()))
				}
				return sb.String(), nil
			}

			prompt := fmt.Sprintf(
				`請審查以下任務的執行結果，指出具體問題並給出改進方向。若結果已完整正確，請明確回應「通過」。

## 任務輸入
%s

## 當前結果
%s

請直接指出問題（如有），或確認通過。`,
				params.Input, params.Result,
			)
			results := external.RunParallel(ctx, agents, prompt, true)
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

func formatFeedback(results []external.Result) string {
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
