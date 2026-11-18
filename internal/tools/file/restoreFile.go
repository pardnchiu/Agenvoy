package file

import (
	"context"
	"encoding/json"
	"strings"

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
Finding the version to restore → file_history; any other change → edit_file.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"version": map[string]any{
					"type":        "integer",
					"description": "Version id from a file_history row; enough on its own, since it names both the file and the state. Takes precedence over task_id.",
				},
				"task_id": map[string]any{
					"type":        "string",
					"description": "Undo a whole task instead: a task id from chat_history, or 'current' for the task running now. Either this or version is required.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "With task_id: return only this file instead of every file the task touched.",
				},
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Version int64  `json:"version"`
				TaskID  string `json:"task_id"`
				Path    string `json:"path"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			var narrow []string
			if path := strings.TrimSpace(params.Path); path != "" {
				narrow = []string{path}
			}
			return filehistory.Restore(ctx, e, params.Version, params.TaskID, narrow)
		},
	})
}
