package schedulerTools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
	"github.com/pardnchiu/agenvoy/internal/session"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
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

	toolRegister.Regist(toolRegister.Def{
		Name:        "list_tasks",
		ReadOnly:    true,
		Description: "列出所有待執行的一次性定時任務，每行一筆，格式為 `{id} {time} {script}`。id 為移除時所需的識別碼。",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			results, err := tasks.ListTasks(scheduler.Get())
			if err != nil {
				return "", err
			}
			if len(results) == 0 {
				return "no onetime tasks", nil
			}
			return strings.Join(results, "\n"), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "get_task",
		ReadOnly:    true,
		Description: "查詢指定 ID 的一次性任務狀態（pending/running/completed/failed）、執行時間、輸出結果與錯誤訊息。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "任務 ID（由 add_task 或 list_tasks 回傳）",
				},
			},
			"required": []string{"id"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			t, ok := tasks.GetTask(scheduler.Get(), params.ID)
			if !ok {
				return "", fmt.Errorf("not found: %s", params.ID)
			}
			lines := []string{
				fmt.Sprintf("id: %s", t.ID),
				fmt.Sprintf("status: %s", t.Status),
				fmt.Sprintf("scheduled_at: %s", t.At.Local().Format("2006-01-02 15:04:05")),
				fmt.Sprintf("script: %s", t.Script),
			}
			if t.StartedAt != nil {
				lines = append(lines, fmt.Sprintf("started_at: %s", t.StartedAt.Local().Format("2006-01-02 15:04:05")))
			}
			if t.FinishedAt != nil {
				lines = append(lines, fmt.Sprintf("finished_at: %s", t.FinishedAt.Local().Format("2006-01-02 15:04:05")))
			}
			if t.Output != "" {
				lines = append(lines, fmt.Sprintf("output: %s", t.Output))
			}
			if t.Err != "" {
				lines = append(lines, fmt.Sprintf("error: %s", t.Err))
			}
			return strings.Join(lines, "\n"), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "remove_task",
		Description: "取消並移除指定 ID 的一次性定時任務，同時刪除對應的腳本檔案。**僅限使用者明確要求刪除排程時才可呼叫，禁止在建立排程流程中主動呼叫。** 若使用者未指定 ID：先呼叫 list_tasks 取得列表，若只有一筆直接移除，若有多筆必須將列表回覆使用者並等待確認。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "任務 ID（由 list_tasks 回傳的第一欄）",
				},
			},
			"required": []string{"id"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if err := tasks.Delete(scheduler.Get(), params.ID); err != nil {
				return "", err
			}
			return fmt.Sprintf("onetime task %s removed", params.ID), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "update_task",
		Description: "修改已存在的一次性任務的執行時間，不刪除腳本、不影響其他設定。若要修改腳本內容請用 update_script。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "要修改的任務 ID（由 list_tasks 回傳）",
				},
				"at": map[string]any{
					"type":        "string",
					"description": "新的執行時間，支援：+5m、+1h30m、15:04、2006-01-02 15:04、RFC3339",
				},
			},
			"required": []string{"id", "at"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
				At string `json:"at"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if err := tasks.Update(scheduler.Get(), params.ID, params.At); err != nil {
				return "", err
			}
			return fmt.Sprintf("task %s updated: scheduled at %s", params.ID, params.At), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "read_script",
		ReadOnly:    true,
		Description: "讀取排程腳本的內容。用於查看或修改腳本前先確認內容。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "腳本檔名（含副檔名，不含路徑），例如 'notify_1741569300.sh'",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if filepath.Base(params.Name) != params.Name {
				return "", fmt.Errorf("must not contain path separator")
			}
			data, err := os.ReadFile(filepath.Join(filesystem.ScriptsDir, params.Name))
			if err != nil {
				return "", fmt.Errorf("os.ReadFile: %w", err)
			}
			return string(data), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "update_script",
		Description: "覆寫已存在的排程腳本內容。用於修改現有腳本，不改變檔名，不影響已設定的排程。先用 read_script 確認內容後再呼叫。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "要覆寫的腳本檔名（含副檔名，不含路徑），例如 'notify_1741569300.sh'",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "新的腳本內容",
				},
			},
			"required": []string{"name", "content"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name    string `json:"name"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if filepath.Base(params.Name) != params.Name {
				return "", fmt.Errorf("must not contain path separator")
			}
			if params.Content == "" {
				return "", fmt.Errorf("content is required")
			}
			if err := filesystem.WriteFile(filepath.Join(filesystem.ScriptsDir, params.Name), params.Content, 0755); err != nil {
				return "", fmt.Errorf("filesystem.WriteFile: %w", err)
			}
			return fmt.Sprintf("script updated: %s", params.Name), nil
		},
	})
}
