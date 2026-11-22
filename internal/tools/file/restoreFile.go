package file

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	filehistory "github.com/pardnchiu/agenvoy/internal/tools/history/file"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registRestoreFile() {
	toolregister.Regist(toolregister.Def{
		Name: "restore_file",
		Description: `Puts a recorded version of a file back, or undoes a whole task by returning every file it touched to before it ran; this also brings back a file remove_file moved aside.
Use for "undo that", "revert to the previous version", "put the file back", "roll back what you just did".
Finding the version to restore → file_history; any other change → write_file.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"version": map[string]any{
					"type":        "integer",
					"description": "Pairs with task_id; set one of the two. Version id from a file_history row, e.g. 42; enough on its own since it names both the file and the state, and wins over task_id. 0 when undoing by task_id.",
				},
				"task_id": map[string]any{
					"type":        "string",
					"description": "Pairs with version and filename: undo a whole task, a task id from chat_history or 'current' for the task running now. '' when restoring by version.",
				},
				"filename": map[string]any{
					"type":        "string",
					"description": "Pairs with task_id: return only this file instead of every file the task touched; absolute, '~/...', or relative to the work directory, e.g. 'src/main.go'. '' to undo every file; unused with version.",
				},
			},
			"required": []string{"task_id", "version", "filename"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Version  int64  `json:"version"`
				TaskID   string `json:"task_id"`
				Filename string `json:"filename"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json: Unmarshal", "", err)
			}

			taskID := strings.TrimSpace(params.TaskID)
			if taskID == "" && params.Version == 0 {
				return "", utilsmsg.New(utilsmsg.Invalid, "task_id/version", "either task_id or version must be set")
			}

			var narrow []string
			if params.Version == 0 && params.Filename != "" {
				absPath, err := boundary.ResolveIn(e.SessionID, e.WorkDir, params.Filename)
				if err != nil {
					return "", utilsmsg.Trace("tools/file/boundary: ResolveIn", "", err)
				}
				narrow = []string{absPath}
			}
			return filehistory.Restore(ctx, e, params.Version, params.TaskID, narrow)
		},
	})
}
