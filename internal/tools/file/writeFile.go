package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registWriteFile() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "write_file",
		Description: "Write content to a file. Creates the file if it does not exist, or overwrites it entirely if it does. Use for new files or full rewrites only — for targeted edits to existing files, use patch_edit instead. Auto git-commits when writing to the skills directory. Set executable: true to save a scheduler script (.sh or .py) — the file is stored in the scripts directory with a timestamp suffix and the returned filename must be passed to add_task or add_cron.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file (relative to project root or absolute). When executable is true, provide only the filename (e.g. 'notify.sh') — path components are ignored.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write to the file",
				},
				"executable": map[string]any{
					"type":        "boolean",
					"description": "If true, saves as an executable script (.sh or .py) to the scheduler scripts directory with a UTC timestamp suffix (e.g. notify_1741569300.sh). The returned filename must be passed to add_task or add_cron.",
				},
			},
			"required": []string{"path", "content"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path       string `json:"path"`
				Content    string `json:"content"`
				Executable bool   `json:"executable"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			if params.Path == "" {
				return "", fmt.Errorf("path is required")
			}
			if params.Content == "" {
				return "", fmt.Errorf("content is required")
			}

			if params.Executable {
				ext := strings.ToLower(filepath.Ext(params.Path))
				if ext != ".sh" && ext != ".py" {
					return "", fmt.Errorf("executable scripts only support .sh or .py")
				}
				base := strings.TrimSuffix(filepath.Base(params.Path), ext)
				uniqueName := fmt.Sprintf("%s_%d%s", base, time.Now().UTC().Unix(), ext)
				absPath := filepath.Join(filesystem.ScriptsDir, uniqueName)
				if err := filesystem.WriteFile(absPath, params.Content, 0755); err != nil {
					return "", fmt.Errorf("filesystem.WriteFile: %w", err)
				}
				return fmt.Sprintf(`script saved. pass "%s" as the script parameter to add_task or add_cron`, uniqueName), nil
			}

			baseDir := e.WorkDir
			if baseDir == "" {
				baseDir = filesystem.DownloadDir
			}
			absPath, err := filesystem.AbsPath(baseDir, params.Path, e.WorkDir != "")
			if err != nil {
				return "", fmt.Errorf("filesystem.AbsPath: %w", err)
			}

			_, statErr := os.Stat(absPath)
			isNew := os.IsNotExist(statErr)

			if err := filesystem.WriteFile(absPath, params.Content, 0644); err != nil {
				return "", fmt.Errorf("filesystem.WriteFile: %w", err)
			}

			if filesystem.IsSkillsDir(absPath) {
				act := "update"
				if isNew {
					act = "add"
				}
				skillName := filesystem.GetSkillName(absPath)
				if err := filesystem.CheckSkillsGit(ctx); err == nil {
					_ = filesystem.CommitSkills(ctx, act, skillName)
				}
			}

			if isNew {
				return fmt.Sprintf("File created successfully at: %s", absPath), nil
			}
			return fmt.Sprintf("The file %s has been updated successfully.", absPath), nil
		},
	})
}
