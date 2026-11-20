// * last modified: 26/11/21
package file

import (
	"context"
	"encoding/json"
	"strings"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
	utilssecret "github.com/pardnchiu/agenvoy/internal/utils/secret"
)

func registWriteFile() {
	toolregister.Regist(toolregister.Def{
		Name: "write_file",
		Description: `Writes one file whole (write) or edits regions of it (patch). Both need a read_files of an existing file this turn, unchanged on disk since.
Use for "write this to a file", "change this part", edit_file / patch_file; only on explicit request or a Skill step, never to park a short answer, tool result or calculation.
Delete → remove_file; undo or revert → restore_file; past versions → file_history; skills → edit_skill; script tools → edit_tool; a long report or page for the user → write_result.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"write", "patch"},
					"description": "write: a first version or a deliberate full replacement. patch: every other change, since a whole-file rewrite discards already-correct text.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "One file per call, absolute or relative to the work directory: '/abs/path/foo.go', '~/notes.md', 'relative/file.md'. A file for the user with no requested location goes under the output directory named in the system prompt; give its full path.",
				},
				"write_content": map[string]any{
					"type":        "string",
					"description": "mode=write: the complete file content, not a diff; empty writes an empty file.",
				},
				"patch_contents": map[string]any{
					"type":        "array",
					"description": "mode=patch: one entry per edit. Every old_string resolves against the on-disk bytes before any edit applies, so entries must not overlap or anchor on text another inserts; merge such edits into one entry.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"old_string": map[string]any{
								"type":        "string",
								"description": "Exact text to replace, indentation included, copied from read_files output without the \"12\\t\" row prefix. Must match once unless replace_all is set; extend it with surrounding lines until it does.",
							},
							"new_string": map[string]any{
								"type":        "string",
								"description": "What old_string becomes; empty deletes it. To insert, start with old_string unchanged, then the new lines.",
							},
							"replace_all": map[string]any{
								"type":        "boolean",
								"description": "Replace every occurrence, e.g. for renames.",
								"default":     false,
							},
						},
						"required": []string{"old_string", "new_string"},
					},
				},
			},
			"required": []string{"mode", "path"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode          string         `json:"mode"`
				Path          string         `json:"path"`
				WriteContent  string         `json:"write_content"`
				PatchContents []patchContent `json:"patch_contents"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json: Unmarshal", "", err)
			}

			var toStrings = func(targets []patchContent) []string {
				list := make([]string, 0, len(targets))
				for _, one := range targets {
					list = append(list, one.NewString)
				}
				return list
			}

			// NOTE: ensure wont to write back mask key to overwrite existing value
			if masked := utilssecret.FindMasked(strings.Join(append([]string{params.WriteContent}, toStrings(params.PatchContents)...), "\n")); masked != "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "write_content/patch_contents", "contains %q, a redacted value from a tool result, not the real text; write the original from inside one run_command (e.g. sh -c with . ./.env) instead", masked)
			}

			mode := strings.ToLower(strings.TrimSpace(params.Mode))
			if mode == "" && params.WriteContent != "" {
				mode = "write"
			} else if mode == "" && len(params.PatchContents) > 0 {
				mode = "patch"
			}

			/**
			 * NOTE:
			 * mode = write and write_content is empty => allow
			 * mode is empty and write_content is empty => error
			 * ensuring support write empty content to file
			 */
			switch mode {
			// NOTE: for ensuring wont send mode empty and write_content empty
			case "":
				return "", utilsmsg.New(utilsmsg.Invalid, "mode", "required; available: write, patch")

			case "write":
				return writeToPath(ctx, e, params.Path, params.WriteContent, "write_file")

			case "patch":
				return patchContents(ctx, e, params.Path, params.PatchContents)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: write, patch", mode)
		},
	})
}
