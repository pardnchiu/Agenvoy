package fileWriter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func Write(ctx context.Context, path, content string, executable bool) (string, error) {
	if executable {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".sh" && ext != ".py" {
			return "", fmt.Errorf("executable scripts only support .sh or .py")
		}

		base := strings.TrimSuffix(filepath.Base(path), ext)
		uniqueName := fmt.Sprintf("%s_%d%s", base, time.Now().UTC().Unix(), ext)
		absPath := filepath.Join(filesystem.ScriptsDir, uniqueName)
		if err := go_utils_filesystem.WriteFile(absPath, content, 0755); err != nil {
			return "", fmt.Errorf("go_utils_filesystem.WriteFile: %w", err)
		}
		return fmt.Sprintf(`script saved. pass "%s" as the script parameter to add_task or add_cron`, uniqueName), nil
	}

	info, err := os.Stat(path)
	isNew := os.IsNotExist(err)
	if err != nil && !isNew {
		return "", fmt.Errorf("os.Stat: %w", err)
	} else if info != nil && info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
	}

	if err := go_utils_filesystem.WriteFile(path, content, 0644); err != nil {
		return "", fmt.Errorf("go_utils_filesystem.WriteFile: %w", err)
	}

	if filesystem.IsSkillsDir(path) {
		act := "update"
		if isNew {
			act = "add"
		}
		skillName := filesystem.GetSkillName(path)
		if err := filesystem.CheckSkillsGit(ctx); err == nil {
			_ = filesystem.CommitSkills(ctx, act, skillName)
		}
	}

	if isNew {
		return fmt.Sprintf("successfully created: %s", path), nil
	}
	return fmt.Sprintf("successfully updated %s", path), nil
}
