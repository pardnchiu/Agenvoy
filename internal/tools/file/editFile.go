package file

import (
	"context"
	"encoding/json"
	"strings"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
	"github.com/pardnchiu/agenvoy/internal/utils/secret"
)

func newStrings(targets []patchTarget) []string {
	list := make([]string, 0, len(targets))
	for _, one := range targets {
		list = append(list, one.NewString)
	}
	return list
}

func registEditFile() {
	toolregister.Regist(toolregister.Def{
		Name:        "edit_file",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Description: `Changes one file on disk: create or replace it (write), or edit regions of it (patch); a write drops its content from history, so check the returned receipt and never read the file back or rewrite it to restore text that only looks missing.
Use for "write this to a file", "change this part", and for write_file / patch_file; only on explicit request or a Skill step, never to park a short answer, a tool result or a calculation.
Deleting a file → remove_file; undoing a change or reverting a version → restore_file; skill files → edit_skill; script tool definitions → edit_tool; a long report or page for the user → write_result; listing past versions → file_history.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"write", "patch"},
					"description": "write: a first version or a deliberate full replacement. patch: every other change, since a whole-file rewrite throws away text that was already correct. Both need a read_files of an existing file in this turn, unchanged on disk since. Omitted: targets → patch, content → write.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "The one file this call acts on, absolute or relative to the work directory: '/abs/path/foo.go', '~/notes.md', 'relative/file.md'. A file made for the user with no location asked for goes under the output directory named in the system prompt; give its full path.",
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
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode    string        `json:"mode"`
				Path    string        `json:"path"`
				Content string        `json:"content"`
				Targets []patchTarget `json:"targets"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			if err := rejectElided(params.Content, params.Targets); err != nil {
				return "", err
			}
			if masked := secret.FindMasked(strings.Join(append([]string{params.Content}, newStrings(params.Targets)...), "\n")); masked != "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "content", "contains %q, a redacted value from a tool result, not the real text; write the original from inside one run_command (e.g. sh -c with . ./.env) instead", masked)
			}

			mode := strings.ToLower(strings.TrimSpace(params.Mode))
			if mode == "" {
				switch {
				case len(params.Targets) > 0:
					mode = "patch"
				case params.Content != "":
					mode = "write"
				default:
					return "", utilsmsg.New(utilsmsg.Invalid, "mode", "required; available: write, patch")
				}
			}

			switch mode {
			case "write":
				return writeFileContent(ctx, e, params.Path, params.Content, "edit_file")
			case "patch":
				return patchFileTargets(ctx, e, params.Path, params.Targets)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: write, patch", mode)
		},
	})
}
