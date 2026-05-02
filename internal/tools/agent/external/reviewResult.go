package externalAgent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

var reviewModelPriority = []struct {
	envKey string
	key    string
}{
	{"ANTHROPIC_API_KEY", "claude@claude-opus-4-6"},
	{"OPENAI_API_KEY", "openai@gpt-5.4"},
	{"GEMINI_API_KEY", "gemini@gemini-3.1-pro-preview"},
	{"ANTHROPIC_API_KEY", "claude@claude-sonnet-4-6"},
}

func selectReviewModelKey() string {
	for _, c := range reviewModelPriority {
		if keychain.Get(c.envKey) != "" {
			return c.key
		}
	}
	return ""
}

func registReviewResult() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "review_result",
		ReadOnly:    true,
		Description: "Review a result against the original task input and return concrete issues and improvement suggestions.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{
					"type":        "string",
					"description": "Original task or question.",
				},
				"result": map[string]any{
					"type":        "string",
					"description": "Result to review.",
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

			modelKey := selectReviewModelKey()
			if modelKey == "" {
				return "內部審查已忽略：無可用模型（需設定 ANTHROPIC_API_KEY / OPENAI_API_KEY / GEMINI_API_KEY 其中之一）。", nil
			}

			out, err := callInternalSend(ctx, modelKey,
				fmt.Sprintf(
					`你是一個品質分析師，請閱讀以下「原始需求」與「產出內容」，以純文字列出產出內容中存在的問題與缺漏，若無問題請回應「通過」。直接輸出分析結論，不呼叫任何 tool。

## 原始需求
%s

## 產出內容
%s`,
					params.Input, params.Result,
				))
			if err != nil {
				return fmt.Sprintf("內部審查失敗（%s）：%s", modelKey, err.Error()), nil
			}

			return fmt.Sprintf("[內部審查 · %s]\n%s", modelKey, out), nil
		},
	})
}
func callInternalSend(ctx context.Context, modelKey, prompt string) (string, error) {
	port := go_pkg_utils.GetWithDefault("PORT", "17989")

	body, err := json.Marshal(map[string]any{
		"content": prompt,
		"model":   modelKey,
	})
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://localhost:"+port+"/v1/send",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("http.NewRequest: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("client.Do: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("io.ReadAll: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		Text  string `json:"text"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("%s", result.Error)
	}
	return result.Text, nil
}
