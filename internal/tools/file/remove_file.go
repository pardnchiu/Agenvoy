// * last modified: 26/11/22
package file

import (
	"context"
	"encoding/json"
	"strings"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registRemoveFile() {
	toolregister.Regist(toolregister.Def{
		Name: "remove_file",
		Description: `Moves one file aside instead of deleting it and returns where it went; restore_file puts it back.
Use for "delete this file", "remove that file", and for delete / rm on a single file; only on explicit request or a Skill step.
Several files at once → run_command rm, which moves them aside the same way; undoing a change or reverting a version → restore_file; a skill or script tool → write_skill / write_tool with mode=remove.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filename": map[string]any{
					"type":        "string",
					"description": "File to remove, absolute or relative to the work directory: '/abs/path/foo.go', '~/notes.md', 'relative/file.md'.",
				},
			},
			"required": []string{"filename"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Filename string `json:"filename"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json: Unmarshal", "", err)
			}

			path := strings.TrimSpace(params.Filename)
			if path == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "filename", "required")
			}
			return MoveToTrash(ctx, e, []string{path}, "remove_file")
		},
	})
}
