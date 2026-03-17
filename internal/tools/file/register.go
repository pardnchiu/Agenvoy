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

	toolRegister.Regist(toolRegister.Def{
		Name:        "search_content",
		Description: "在檔案內容中搜尋模式。返回符合的行及其檔案路徑和行號。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "要搜尋的文字或正規表示式模式",
				},
				"file_pattern": map[string]any{
					"type":        "string",
					"description": "可選的 glob 模式以篩選檔案（例如 '*.go'）",
				},
			},
			"required": []string{"pattern"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Pattern     string `json:"pattern"`
				FilePattern string `json:"file_pattern"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return search(e, params.Pattern, params.FilePattern)
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "search_history",
		Description: "在當前 session 的對話歷史（history.json）中搜尋關鍵字，返回包含該字詞的完整對話內容（含 role 與 content）。支援時間範圍過濾，僅返回指定時間內的紀錄。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "要搜尋的關鍵字（不區分大小寫，literal 字串比對）",
				},
				"time_range": map[string]any{
					"type":        "string",
					"enum":        []string{"1d", "7d", "1m", "1y"},
					"description": "時間範圍過濾（1d=1天、7d=7天、1m=30天、1y=365天）。預設先用 1d，無結果再用 7d，仍無結果才考慮 1m/1y",
				},
			},
			"required": []string{"keyword"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Keyword   string `json:"keyword"`
				TimeRange string `json:"time_range"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return searchHistory(e.SessionID, params.Keyword, params.TimeRange)
		},
	})

	registWriteFile()
	registWriteScript()
	registPatchEdit()

	toolRegister.Regist(toolRegister.Def{
		Name:        "get_tool_error",
		Description: "透過 hash 查詢工具執行錯誤的詳細資訊。當工具回傳 'no data: {hash}' 時，使用此工具取得完整錯誤內容（tool_name、args、error message）。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"hash": map[string]any{
					"type":        "string",
					"description": "錯誤識別碼（8字元 hex），來自工具回傳的 'no data: {hash}'",
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
		Description: "記錄工具執行錯誤的決策經驗到永久儲存，供後續 session 遇到相同問題時直接參考行動方案。確認根因與處理方式後呼叫。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool_name": map[string]any{
					"type":        "string",
					"description": "發生錯誤的 tool 名稱",
				},
				"keywords": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "用於搜尋比對的關鍵字列表（工具名稱、錯誤類型、相關參數特徵等），越具體越好",
				},
				"symptom": map[string]any{
					"type":        "string",
					"description": "觀察到的現象描述（工具回傳了什麼、發生了什麼異常）",
				},
				"cause": map[string]any{
					"type":        "string",
					"description": "根本原因分析（可選，確認後填入）",
				},
				"action": map[string]any{
					"type":        "string",
					"description": "採取的具體行動（例：改用英文關鍵字重試、換用 search_web 替代）",
				},
				"outcome": map[string]any{
					"type":        "string",
					"description": "行動結果（resolved / failed / partial，可加說明）",
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
		Description: "查詢歷史工具錯誤的決策經驗記錄。遇到工具異常時先呼叫，取得過去相同情境的根因與行動方案，直接套用。搜尋範圍涵蓋 keywords、symptom、cause、tool_name。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "搜尋關鍵字（tool 名稱、錯誤現象、相關參數特徵，不區分大小寫）",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "最多返回筆數，預設 4，最大 16",
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
