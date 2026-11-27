package variant

import (
	"context"
	"encoding/json"
	"strings"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registEditSkill() {
	toolregister.Regist(toolregister.Def{
		Name: "write_skill",
		Description: `Edits files under the skills directory: create or rewrite one (write), replace a string inside one (patch), trash a whole skill (remove), or copy a whole skill directory in from elsewhere, binary files included (copy); returns the affected path.
Use for "edit this skill", "add a skill file", "this skill needs adjusting", "delete that skill", "copy this skill into Agenvoy", and for patch_skill / remove_skill.
Running a skill → run_skill; building a new skill from scratch → the skill-creator skill; script tools → write_tool; any other file → write_file.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"write", "patch", "remove", "copy"},
					"description": "write: a first version or a full rewrite of one file (uses path). patch: a targeted edit (uses path). remove: moves the whole skill directory to .Trash (uses name). copy: copies the skill directory at from into the skills directory as name (fails if name exists). Omitted: content → write, old_string → patch, from → copy, name alone → remove.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "mode=write / mode=patch: path relative to the skills directory, staying inside it, e.g. 'my-skill/SKILL.md', 'my-skill/scripts/01.md'.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "mode=write: the complete file content, not a diff.",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "mode=patch: exact string to replace, including indentation. Without replace_all only its first occurrence changes, so include enough surrounding text to make it unique.",
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
				"name": map[string]any{
					"type":        "string",
					"description": "mode=remove: skill directory name, a single path segment, e.g. 'my-skill'. mode=copy: target directory name; omitted uses the source directory's name.",
				},
				"from": map[string]any{
					"type":        "string",
					"description": "mode=copy: the skill directory to copy (it must contain SKILL.md), absolute or relative to the work directory, e.g. '~/.claude/skills/my-skill'.",
				},
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode       string `json:"mode"`
				Path       string `json:"path"`
				Content    string `json:"content"`
				OldString  string `json:"old_string"`
				NewString  string `json:"new_string"`
				ReplaceAll bool   `json:"replace_all"`
				Name       string `json:"name"`
				From       string `json:"from"`
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
				case strings.TrimSpace(params.From) != "":
					mode = "copy"
				case strings.TrimSpace(params.Name) != "":
					mode = "remove"
				default:
					return "", utilsmsg.New(utilsmsg.Invalid, "mode", "required; available: write, patch, remove, copy")
				}
			}

			switch mode {
			case "write":
				return writeSkillSrite(ctx, e, params.Path, params.Content)
			case "patch":
				return writeSkillPatch(ctx, e, params.Path, params.OldString, params.NewString, params.ReplaceAll)
			case "remove":
				return writeSkillRemove(ctx, e, params.Name)
			case "copy":
				return writeSkillCopy(e, params.From, params.Name)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: write, patch, remove, copy", mode)
		},
	})
}
