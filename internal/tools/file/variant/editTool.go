package variant

import (
	"context"
	"encoding/json"
	"strings"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registEditTool() {
	toolregister.Regist(toolregister.Def{
		Name:        "edit_tool",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  false,
		Description: `Edits a script tool's files (tool.json schema, script.py runtime): create or overwrite one (write), replace a string inside one (patch), or trash the whole tool (remove); returns the affected path.
Use for "build a tool for this", "fix this tool", "this tool is broken", "delete that tool", and for write_tool / patch_tool / remove_tool.
Finding an existing tool → find_tools; after writing tool.json and script.py, check it with test_tool before calling it; skill files → edit_skill; any other file → edit_file.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"write", "patch", "remove"},
					"description": "write: a first version or a full replacement of one file. patch: fixing an existing tool, e.g. after test_tool failed. remove: moves the whole tool directory to .Trash. Omitted: content → write, old_string → patch; remove is never inferred.",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Tool directory name in snake_case without the 'script_' prefix the runtime adds, e.g. 'ip_geolocation_lookup'. patch and remove need an existing tool.",
				},
				"tag": map[string]any{
					"type":        "string",
					"enum":        []string{"json", "script"},
					"description": "Required for write and patch: which file. json: tool.json (schema). script: script.py (runtime). One file per call, so a new tool takes two writes.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "mode=write: the complete file content, not a diff.",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "mode=patch: exact string to replace. Without replace_all only its first occurrence changes, so include enough surrounding text to make it unique.",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "mode=patch: replacement string; empty deletes old_string.",
				},
				"replace_all": map[string]any{
					"type":        "boolean",
					"description": "mode=patch: replace every occurrence instead of only the first.",
					"default":     false,
				},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode       string `json:"mode"`
				Name       string `json:"name"`
				Tag        string `json:"tag"`
				Content    string `json:"content"`
				OldString  string `json:"old_string"`
				NewString  string `json:"new_string"`
				ReplaceAll bool   `json:"replace_all"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			mode := strings.ToLower(strings.TrimSpace(params.Mode))
			if mode == "" {
				switch {
				case params.Content != "":
					mode = "write"
				case params.OldString != "":
					mode = "patch"
				default:
					return "", utilsmsg.New(utilsmsg.Invalid, "mode", "required; available: write, patch, remove")
				}
			}

			switch mode {
			case "write":
				return writeToolFile(ctx, e, params.Name, params.Tag, params.Content)
			case "patch":
				return patchToolFile(ctx, e, params.Name, params.Tag, params.OldString, params.NewString, params.ReplaceAll)
			case "remove":
				return removeToolDir(ctx, e, params.Name)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: write, patch, remove", mode)
		},
	})
}
