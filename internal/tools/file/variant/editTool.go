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
		Description: `The tool definitions themselves: create or overwrite one (write), fix an exact string inside one (patch), trash an obsolete one (remove).
Use for 建一個工具 / 修工具 / 這個工具壞了, and for write_tool / patch_tool / remove_tool.
The build flow is write → test_tool → call it. Finding an existing tool → find_tools; skill files → edit_skill.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"write", "patch", "remove"},
					"description": "write: a first version or a full replacement. patch: fixing a broken tool after test_tool failed. remove: trashes the directory, recoverable from .Trash. Omitted: content → write, old_string → patch; remove is never inferred.",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Snake_case, no 'script_' prefix — the runtime adds it. e.g. 'ip_geolocation_lookup'.",
				},
				"tag": map[string]any{
					"type":        "string",
					"enum":        []string{"json", "script"},
					"description": "mode=write / mode=patch: which file — json = tool.json (schema), script = script.py (runtime).",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "mode=write: the complete file content, not a diff.",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "mode=patch: exact string to replace. Must be unique in the target file.",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "mode=patch: replacement string. Empty string deletes old_string.",
				},
				"replace_all": map[string]any{
					"type":        "boolean",
					"description": "mode=patch: replace every occurrence instead of the single unique one.",
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
