package externalAgent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/external"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registCrossReviewWithExternalAgents() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "cross_review_with_external_agents",
		AlwaysAllow: true,
		Timeout:     15 * time.Minute,
		Description: "Fan out a deliverable to all installed external CLIs in parallel for cross-review. Use when user explicitly requests cross-check or multi-model verification. Requires ExternalAgents; if none configured, use review_result instead.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{
					"type":        "string",
					"description": "Original task or question.",
				},
				"result": map[string]any{
					"type":        "string",
					"description": "Result to be reviewed.",
				},
			},
			"required": []string{
				"input",
				"result",
			},
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
				return `外部驗證已忽略：PATH 未偵測到任何外部 CLI（codex / gh / claude / gemini）。請先安裝對應 CLI。`, nil
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
			preview := go_pkg_utils.TruncateString(r.Output, 512)
			sb.WriteString(fmt.Sprintf("[%s]\n%s\n\n", r.Agent, preview))
		}
	}
	return sb.String()
}
