package file

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registPatchEdit() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "patch_edit",
		Description: "Edit a file by exact string match. Replaces the first occurrence unless replace_all is set. Safer than write_file for targeted changes.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file. Absolute path preferred; relative paths resolve against the work directory shown in the system prompt. `~` expands to user home.",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "Exact string to replace (must match precisely, including indentation). The edit will fail if not found or if it matches multiple locations without replace_all.",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "Replacement string. Use empty string to delete old_string.",
				},
				"replace_all": map[string]any{
					"type":        "boolean",
					"description": "If true, replace all occurrences. Use when renaming variables or repeated patterns. Defaults to false.",
				},
			},
			"required": []string{"path", "old_string", "new_string"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path       string `json:"path"`
				OldString  string `json:"old_string"`
				NewString  string `json:"new_string"`
				ReplaceAll bool   `json:"replace_all"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			if params.OldString == params.NewString {
				return "", fmt.Errorf("old_string and new_string are identical: no changes to make")
			}

			content, absPath, err := readFile(e, params.Path)
			if err != nil {
				return "", fmt.Errorf("file.readFile: %w", err)
			}

			count := strings.Count(content, params.OldString)
			if count == 0 {
				return "", fmt.Errorf("%s is not found in %s", params.OldString, absPath)
			}
			if !params.ReplaceAll && count > 1 {
				return "", fmt.Errorf("old_string matches %d locations in %s — add more context to make it unique, or set replace_all to true", count, absPath)
			}

			newContent := applyEdit(content, params.OldString, params.NewString, params.ReplaceAll)
			if err := filesystem.WriteFile(absPath, newContent, 0644); err != nil {
				return "", fmt.Errorf("filesystem.WriteFile: %w", err)
			}

			if filesystem.IsSkillsDir(absPath) {
				skillName := filesystem.GetSkillName(absPath)
				if err := filesystem.CheckSkillsGit(ctx); err == nil {
					_ = filesystem.CommitSkills(ctx, "update", skillName)
				}
			}

			return fmt.Sprintf("successfully updated %s", absPath), nil
		},
	})
}

func applyEdit(content, oldString, newString string, replaceAll bool) string {
	target := oldString
	if newString == "" && !strings.HasSuffix(oldString, "\n") && strings.Contains(content, oldString+"\n") {
		target = oldString + "\n"
	}
	if replaceAll {
		return strings.ReplaceAll(content, target, newString)
	}
	return strings.Replace(content, target, newString, 1)
}
