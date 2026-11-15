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
		Name:        "remove_file",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Description: `Moves one file aside instead of deleting it and returns where it went; restore_file puts it back.
Use for "delete this file", "remove that file", and for delete / rm on a single file; only on explicit request or a Skill step.
Undoing a change or reverting a version → restore_file; a skill or script tool → edit_skill / edit_tool with mode=remove.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The file to remove, absolute or relative to the work directory: '/abs/path/foo.go', '~/notes.md', 'relative/file.md'.",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			path := strings.TrimSpace(params.Path)
			if path == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "path", "required")
			}
			return RemoveToTrash(ctx, e, []string{path}, "remove_file")
		},
	})
}
