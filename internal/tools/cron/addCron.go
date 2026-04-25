package cronTools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	"github.com/pardnchiu/agenvoy/internal/session"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registAddCron() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "add_cron",
		Description: "新增重複性定時任務（recurring cron job）。使用標準 cron 表達式（`* * * * *`，依序為 分 時 日 月 週），每次到達排程時間即執行腳本。任務持久保存，重啟後仍會繼續執行。【必須先呼叫 write_script，將回傳的實際檔名填入 script；多個不同時間的 cron 可共用同一個 script 檔名，無需重複呼叫 write_script】若設定 discord_channel_id，每次執行完畢後會將輸出傳送到指定 Discord 頻道。回傳結果包含 task ID。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cron_expr": map[string]any{
					"type":        "string",
					"description": "標準 cron 表達式，5 個欄位空格分隔：`{分} {時} {日} {月} {週}`。支援 `*`（任意）、`*/n`（每 n 單位）、`n`（精確值）、`n,m`（列舉）、`n-m`（範圍）。範例：`* * * * *`（每分鐘）、`0 9 * * 1`（每週一早上 9 點）、`*/5 * * * *`（每 5 分鐘）",
				},
				"script": map[string]any{
					"type":        "string",
					"description": "write_script 回傳的實際檔名（含 timestamp 後綴），例如 'backup_1741569300.sh'",
				},
				"channel_id": map[string]any{
					"type":        "string",
					"description": "（可選）每次執行完畢後要回傳結果的 Discord 頻道 ID",
				},
			},
			"required": []string{"cron_expr", "script"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				CronExpr  string `json:"cron_expr"`
				Script    string `json:"script"`
				ChannelID string `json:"channel_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if params.ChannelID == "" {
				if id, err := session.GetChannelID(e.SessionID); err == nil {
					params.ChannelID = id
				}
			}
			mgr := scheduler.Get()
			if mgr == nil {
				msg, err := crons.AddToFile(params.CronExpr, params.Script, params.ChannelID)
				if err != nil {
					return "", err
				}
				return msg + "\n排程已寫入檔案，但需啟動 app 才能實際執行。", nil
			}
			return crons.Add(mgr, params.CronExpr, params.Script, params.ChannelID)
		},
	})

}
