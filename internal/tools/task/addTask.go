package taskTools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
	"github.com/pardnchiu/agenvoy/internal/session"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registerAddTask() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "add_task",
		Description: "設定一次性定時任務，到達指定時間時執行腳本，執行後自動從排程中移除並刪除對應腳本檔案。【必須先呼叫 write_script，並將其回傳的實際檔名填入 script】若設定 discord_channel_id，腳本執行完畢後會將輸出結果自動傳送到該 Discord 頻道。回傳結果包含 task ID。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"at": map[string]any{
					"type":        "string",
					"description": "執行時間，支援：+5m（5分鐘後）、+1h30m（1.5小時後）、15:04（今天指定時間）、2006-01-02 15:04（指定日期時間）、RFC3339",
				},
				"script": map[string]any{
					"type":        "string",
					"description": "write_script 回傳的實際檔名（含 timestamp 後綴），例如 'open_pardn_io_1741569300.sh'",
				},
				"channel_id": map[string]any{
					"type":        "string",
					"description": "（可選）腳本完成後要回傳結果的 Discord 頻道 ID。填入後，腳本的 stdout/stderr 輸出會自動送至該頻道。",
				},
			},
			"required": []string{"at", "script"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				At        string `json:"at"`
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
				msg, err := tasks.AddToFile(params.At, params.Script, params.ChannelID)
				if err != nil {
					return "", err
				}
				return msg + "\n排程已寫入檔案，但需啟動 app 才能實際執行。", nil
			}
			return tasks.Add(mgr, params.At, params.Script, params.ChannelID)
		},
	})

}
