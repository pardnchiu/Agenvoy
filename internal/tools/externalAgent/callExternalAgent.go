package externalAgent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registCallExternalAgent() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "call_external_agent",
		Description: `將超出本專案 tool 能力範圍的請求完整委派給指定外部 AI agent，由外部 agent 直接生成最終結果。適用於：請求涉及本專案未支援的工具或操作，且無法透過現有 tool 組合完成。⚠ 外部 agent 在獨立環境執行，無法使用本專案 tool。⚠ 禁止因「不確定用哪個 tool」而 fallback 到此 tool。`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent": map[string]any{
					"type":        "string",
					"description": "要呼叫的外部 agent，必須從已宣告的可用清單中選擇",
					"enum":        []string{"codex", "copilot", "claude"},
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "完整的任務描述，包含所有必要的上下文與要求",
				},
			},
			"required": []string{"agent", "prompt"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Agent  string `json:"agent"`
				Prompt string `json:"prompt"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			if !slices.Contains(GetAgents(), params.Agent) {
				return fmt.Sprintf(
					"外部呼叫已忽略：%s 未宣告（請設定 EXTERNAL_%s=true）。",
					params.Agent, strings.ToUpper(params.Agent),
				), nil
			}

			if err := checkCLI(params.Agent); err != nil {
				return fmt.Sprintf(
					"外部呼叫已忽略（%s: %s）",
					params.Agent, err.Error(),
				), nil
			}

			out, err := runOne(ctx, params.Agent, params.Prompt)
			if err != nil {
				return fmt.Sprintf(
					"外部呼叫失敗（%s: %s）",
					params.Agent, err.Error(),
				), nil
			}

			return fmt.Sprintf("[外部呼叫 · %s]\n%s", params.Agent, out), nil
		},
	})
}

func runOne(ctx context.Context, agent, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	switch agent {
	case "codex":
		cmd = exec.CommandContext(ctx, "codex", "exec", prompt)
	case "copilot":
		cmd = exec.CommandContext(ctx, "gh", "copilot", "-p", prompt)
	case "claude":
		cmd = exec.CommandContext(ctx, "claude", "-p", prompt)
	default:
		return "", fmt.Errorf("%s not supported", agent)
	}

	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		if output != "" {
			return "", fmt.Errorf("%s: %s", err.Error(), output)
		}
		return "", err
	}
	if output == "" {
		return "", fmt.Errorf("empty response from %s", agent)
	}
	return output, nil
}
