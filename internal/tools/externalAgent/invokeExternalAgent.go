package externalAgent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/external"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registInvokeExternalAgent() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "invoke_external_agent",
		ReadOnly: true,
		Description: `
呼叫外部 CLI agent（codex／copilot／claude）在隔離環境執行，取得不同 harness 的獨立第二意見；**非能力補位，非 fallback**。

vs invoke_subagent：後者走本專案 exec 引擎、共用 tool／session／registry；此 tool 完全隔離，外部 agent 無法存取本專案任何 tool。

僅在使用者指名、或需與自身結論交叉比對時使用。prompt 必須自包含（外部 agent 看不到本 session 歷史）。`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"provider": map[string]any{
					"type":        "string",
					"description": "要呼叫的外部 agent，必須從已宣告的可用清單中選擇",
					"enum":        []string{"codex", "copilot", "claude"},
				},
				"task": map[string]any{
					"type":        "string",
					"description": "完整自包含的任務描述：外部 agent 看不到本專案對話歷史／session／檔案，所有必要上下文、輸入資料、要求格式都必須明寫在 prompt 中",
				},
				"readonly": map[string]any{
					"type":        "boolean",
					"description": "是否限制為唯讀模式（禁止寫檔／執行破壞性指令）。預設 true；需要外部 agent 實際修改檔案時才設 false。",
					"default":     true,
				},
			},
			"required": []string{"provider", "task"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Provider string `json:"provider"`
				Task     string `json:"task"`
				ReadOnly *bool  `json:"readonly,omitempty"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			readOnly := true
			if params.ReadOnly != nil {
				readOnly = *params.ReadOnly
			}

			if !slices.Contains(external.Agents(), params.Provider) {
				return fmt.Sprintf(
					"外部呼叫已忽略：%s 未宣告（請設定 EXTERNAL_%s=true）。",
					params.Provider, strings.ToUpper(params.Provider),
				), nil
			}

			if err := external.Check(params.Provider); err != nil {
				return fmt.Sprintf(
					"外部呼叫已忽略（%s: %s）",
					params.Provider, err.Error(),
				), nil
			}

			out, err := external.Run(ctx, params.Provider, params.Task, readOnly)
			if err != nil {
				return fmt.Sprintf(
					"外部呼叫失敗（%s: %s）",
					params.Provider, err.Error(),
				), nil
			}

			return fmt.Sprintf("[外部呼叫 · %s]\n%s", params.Provider, out), nil
		},
	})
}
