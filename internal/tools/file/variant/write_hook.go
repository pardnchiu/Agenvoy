package variant

import (
	"context"
	"encoding/json"
	"strings"

	agentsexechook "github.com/pardnchiu/agenvoy/internal/agents/exec/hook"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registWriteHook() {
	toolregister.Regist(toolregister.Def{
		Name:        "write_hook",
		AlwaysAllow: true,
		Description: `Edits a hook script under the hooks directory: create or rewrite it (write), replace a string inside the installed one (patch), or trash the installed one (remove); returns the affected path.
Use for "block X before sending", "stop when tokens exceed N", "notify me when a task finishes", "change / delete that hook" — read reasoning_guide(hook) and read_hook first; patch and remove need the hook read_hook shows under current; after write or patch, run test_hook.
Skills → edit_skill; script tools → edit_tool; any other file → write_file.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"write", "patch", "remove"},
					"description": "write: a first version when read_hook shows current (none), or a full rewrite. patch: a targeted edit of the installed hook, only when read_hook shows one under current. remove: moves the installed hook to .Trash. Omitted: content → write, old_string → patch, name alone → remove.",
				},
				"name": map[string]any{
					"type":        "string",
					"enum":        agentsexechook.Names(),
					"description": "Hook state; the file is <name>.py or <name>.js in the hooks directory.",
				},
				"lang": map[string]any{
					"type":        "string",
					"enum":        []string{"py", "js"},
					"default":     "py",
					"description": "mode=write for a state with no hook yet: js only when the user asks for JavaScript. An existing hook keeps its own language (.py over .js).",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "mode=write: the complete script in the hook's language, not a diff.",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "mode=patch: exact string to replace, copied from the current script read_hook returned, including indentation. Without replace_all only its first occurrence changes, so include enough surrounding text to make it unique.",
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
				Lang       string `json:"lang"`
				Content    string `json:"content"`
				OldString  string `json:"old_string"`
				NewString  string `json:"new_string"`
				ReplaceAll bool   `json:"replace_all"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json: Unmarshal", "", err)
			}

			mode := strings.ToLower(strings.TrimSpace(params.Mode))
			if mode == "" {
				switch {
				case params.Content != "":
					mode = "write"
				case params.OldString != "":
					mode = "patch"
				default:
					mode = "remove"
				}
			}

			switch mode {
			case "write":
				return writeHookWrite(ctx, e, params.Name, params.Lang, params.Content)
			case "patch":
				return writeHookPatch(ctx, e, params.Name, params.OldString, params.NewString, params.ReplaceAll)
			case "remove":
				return writeHookRemove(ctx, e, params.Name)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: write, patch, remove", mode)
		},
	})
}
