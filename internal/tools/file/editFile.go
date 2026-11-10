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

func registEditFile() {
	toolregister.Regist(toolregister.Def{
		Name:        "edit_file",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Description: `Changes one file on disk: create or replace it (write), edit regions of it (patch), move it aside (remove), or put a recorded version back (restore); a write drops its content from history, so check the returned receipt and never read the file back or rewrite it to restore text that only looks missing.
Use for "write this to a file", "change this part", "delete this file", "undo that", "revert to the previous version", and for write_file / patch_file / remove_file / restore_file / delete; only on explicit request or a Skill step, never to park a short answer, a tool result or a calculation.
Skill files → edit_skill; script tool definitions → edit_tool; a long report or page for the user → write_result; listing past versions → file_history.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"write", "patch", "remove", "restore"},
					"description": "write: a first version or a deliberate full replacement. patch: every other change, since a whole-file rewrite throws away text that was already correct. remove: moves the file aside, restorable with mode=restore. restore: put a recorded version back. write and patch on an existing file need a read_files of it in this turn, unchanged on disk since. Omitted: targets → patch, content → write; remove and restore are never inferred.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "The one file this call acts on, absolute or relative to the work directory: '/abs/path/foo.go', '~/notes.md', 'relative/file.md'. Required for write, patch and remove; on restore it narrows a task_id undo to that file. A file made for the user with no location asked for goes under the output directory named in the system prompt; give its full path.",
					"default":     "",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "mode=write: the complete file content, not a diff.",
				},
				"targets": map[string]any{
					"type":        "array",
					"description": "mode=patch: the edits to that file, each {old_string, new_string}. Every old_string resolves against the bytes on disk before any edit applies, so targets must not overlap and one target cannot anchor on text another inserts; merge such edits into one target.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"old_string": map[string]any{
								"type":        "string",
								"description": "Exact text to replace, indentation included, copied from read_files output without the \"12\\t\" row prefix. Must match once unless replace_all is set; extend it with surrounding lines until it does.",
							},
							"new_string": map[string]any{
								"type":        "string",
								"description": "What old_string becomes. Empty deletes it. To insert, start with old_string unchanged and append the new lines after it.",
							},
							"replace_all": map[string]any{
								"type":        "boolean",
								"description": "Replace every occurrence instead of requiring a unique match, e.g. for renames.",
								"default":     false,
							},
						},
						"required": []string{"old_string", "new_string"},
					},
				},
				"version": map[string]any{
					"type":        "integer",
					"description": "mode=restore: version id from a file_history row; enough on its own, since it names both the file and the state. Takes precedence over task_id.",
				},
				"task_id": map[string]any{
					"type":        "string",
					"description": "mode=restore: undo a whole task instead, returning every file it touched (or only path, when set) to before it ran; 'current' means the task running now. Either this or version is required.",
				},
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode    string        `json:"mode"`
				Path    string        `json:"path"`
				Content string        `json:"content"`
				Targets []patchTarget `json:"targets"`
				Version int64         `json:"version"`
				TaskID  string        `json:"task_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			if err := rejectElided(params.Content, params.Targets); err != nil {
				return "", err
			}

			mode := strings.ToLower(strings.TrimSpace(params.Mode))
			if mode == "" {
				switch {
				case len(params.Targets) > 0:
					mode = "patch"
				case params.Content != "":
					mode = "write"
				default:
					return "", utilsmsg.New(utilsmsg.Invalid, "mode", "required; available: write, patch, remove, restore")
				}
			}

			switch mode {
			case "write":
				return writeFileContent(ctx, e, params.Path, params.Content, "edit_file")
			case "patch":
				return patchFileTargets(ctx, e, params.Path, params.Targets)
			case "remove":
				path := strings.TrimSpace(params.Path)
				if path == "" {
					return "", utilsmsg.New(utilsmsg.Invalid, "path", "required when mode=remove")
				}
				return RemoveToTrash(ctx, e, []string{path}, "edit_file")
			case "restore":
				var narrow []string
				if path := strings.TrimSpace(params.Path); path != "" {
					narrow = []string{path}
				}
				return filehistory.Restore(ctx, e, params.Version, params.TaskID, narrow)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: write, patch, remove, restore", mode)
		},
	})
}
