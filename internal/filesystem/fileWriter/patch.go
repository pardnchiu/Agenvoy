package fileWriter

import (
	"context"
	"fmt"
	"os"
	"strings"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const (
	maxReadSize = 1 << 20
)

func Patch(ctx context.Context, path, old, new string, replaceAll bool) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("os.Stat: %w", err)
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("os.ReadFile: %w", err)
	}

	fileContent := string(fileBytes)
	matchCount := strings.Count(fileContent, old)
	if matchCount == 0 {
		return "", fmt.Errorf("%s is not found in %s", old, path)
	}

	newContent := old
	if new == "" && !strings.HasSuffix(old, "\n") && strings.Contains(fileContent, old+"\n") {
		newContent = old + "\n"
	}
	if replaceAll {
		newContent = strings.ReplaceAll(fileContent, newContent, new)
	} else {
		newContent = strings.Replace(fileContent, newContent, new, 1)
	}

	if err := go_utils_filesystem.WriteFile(path, newContent, 0644); err != nil {
		return "", fmt.Errorf("go_utils_filesystem.WriteFile: %w", err)
	}

	if filesystem.IsSkillsDir(path) {
		skillName := filesystem.GetSkillName(path)
		if err := filesystem.CheckSkillsGit(ctx); err == nil {
			_ = filesystem.CommitSkills(ctx, "update", skillName)
		}
	}
	return fmt.Sprintf("successfully updated %s", path), nil
}
