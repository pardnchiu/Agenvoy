package schedulerTools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem/sessionManager"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "add_cron",
		Description: "新增重複性定時任務（recurring cron job）。使用標準 cron 表達式（`* * * * *`，依序為 分 時 日 月 週），每次到達排程時間即執行腳本。任務持久保存，重啟後仍會繼續執行。【必須先呼叫 write_script，將回傳的實際檔名填入 script】若設定 discord_channel_id，每次執行完畢後會將輸出傳送到指定 Discord 頻道。",
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
				channelID, err := sessionManager.GetChannelID(e.SessionID)
				if err != nil {
					return "", fmt.Errorf("GetChannelID: %w", err)
				}
				params.ChannelID = channelID
			}
			mgr := scheduler.Get()
			if mgr == nil {
				return "", fmt.Errorf("scheduler not initialized")
			}
			if err := mgr.AddCron(params.CronExpr, params.Script, params.ChannelID); err != nil {
				return "", err
			}
			return fmt.Sprintf("cron task added: %s %s", params.CronExpr, params.Script), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "list_crons",
		Description: "列出所有目前啟用中的重複性 cron 任務，每行一筆，格式為 `{index}. {cron_expr} {script} [{channel_id}]`。index 為移除時所需的編號。",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			mgr := scheduler.Get()
			if mgr == nil {
				return "", fmt.Errorf("scheduler not initialized")
			}
			tasks := mgr.ListCronTasks()
			if len(tasks) == 0 {
				return "no cron tasks", nil
			}
			return strings.Join(tasks, "\n"), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "remove_cron",
		Description: "移除指定編號的重複性 cron 任務。流程：1. 先呼叫 list_crons；2. 若只有一個任務，直接移除；3. 若有多個任務，必須將列表回覆給使用者並詢問要移除哪一個，等待使用者明確指定後才呼叫此工具。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"index": map[string]any{
					"type":        "integer",
					"description": "任務編號（由 list_crons 回傳的序號，從 1 開始）",
				},
			},
			"required": []string{"index"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			mgr := scheduler.Get()
			if mgr == nil {
				return "", fmt.Errorf("scheduler not initialized")
			}
			if err := mgr.RemoveCronTask(params.Index); err != nil {
				return "", err
			}
			return fmt.Sprintf("cron task #%d removed", params.Index), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "add_task",
		Description: "設定一次性定時任務，到達指定時間時執行腳本，執行後自動從排程中移除並刪除對應腳本檔案。【必須先呼叫 write_script，並將其回傳的實際檔名填入 script】若設定 discord_channel_id，腳本執行完畢後會將輸出結果自動傳送到該 Discord 頻道。",
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
				channelID, err := sessionManager.GetChannelID(e.SessionID)
				if err != nil {
					return "", fmt.Errorf("GetChannelID: %w", err)
				}
				params.ChannelID = channelID
			}
			mgr := scheduler.Get()
			if mgr == nil {
				return "", fmt.Errorf("scheduler not initialized")
			}
			return mgr.AddTask(params.At, params.Script, params.ChannelID)
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "list_tasks",
		Description: "列出所有待執行的一次性定時任務，每行一筆，格式為 `{index}. {RFC3339_time} {script} [{channel_id}]`。index 為移除時所需的編號。",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			mgr := scheduler.Get()
			if mgr == nil {
				return "", fmt.Errorf("scheduler not initialized")
			}
			tasks := mgr.ListTasks()
			if len(tasks) == 0 {
				return "no onetime tasks", nil
			}
			return strings.Join(tasks, "\n"), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "remove_task",
		Description: "取消並移除指定編號的一次性定時任務，同時刪除對應的腳本檔案。流程：1. 先呼叫 list_tasks；2. 若只有一個任務，直接移除；3. 若有多個任務，必須將列表回覆給使用者並詢問要移除哪一個，等待使用者明確指定後才呼叫此工具。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"index": map[string]any{
					"type":        "integer",
					"description": "任務編號（由 list_tasks 回傳的序號，從 1 開始）",
				},
			},
			"required": []string{"index"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			mgr := scheduler.Get()
			if mgr == nil {
				return "", fmt.Errorf("scheduler not initialized")
			}
			if err := mgr.RemoveTask(params.Index); err != nil {
				return "", err
			}
			return fmt.Sprintf("onetime task #%d removed", params.Index), nil
		},
	})
}
