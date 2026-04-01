package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"os/exec"
)

func CheckSkillsGit(ctx context.Context) error {
	if err := os.MkdirAll(SkillsDir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}

	dir := filepath.Join(SkillsDir, ".git")
	if _, err := os.Stat(dir); err == nil {
		return nil
	}

	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = SkillsDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cmd.CombinedOutput: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func CommitSkills(ctx context.Context, act, skillName string) error {
	date := time.Now().Format("20060102")
	message := fmt.Sprintf("%s_%s_%s", act, skillName, date)

	cmd := exec.CommandContext(ctx, "git", "add", "-A")
	cmd.Dir = SkillsDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add.CombinedOutput: %w: %s", err, strings.TrimSpace(string(out)))
	}

	status := exec.CommandContext(ctx, "git", "status", "--porcelain")
	status.Dir = SkillsDir
	statusOut, err := status.CombinedOutput()
	if err != nil {
		return fmt.Errorf("status.CombinedOutput: %w: %s", err, strings.TrimSpace(string(statusOut)))
	}
	if strings.TrimSpace(string(statusOut)) == "" {
		return nil
	}

	cmd = exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = SkillsDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("commit.CombinedOutput: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func LogSkills(ctx context.Context, limit int) (string, error) {
	if limit <= 0 {
		limit = 20
	}
	cmd := exec.CommandContext(ctx, "git", "log", "--oneline", fmt.Sprintf("-n%d", limit))
	cmd.Dir = SkillsDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cmd.CombinedOutput: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func RollbackSkills(ctx context.Context, commit string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "reset", "--hard", commit)
	cmd.Dir = SkillsDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cmd.CombinedOutput: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func IsSkillsDir(path string) bool {
	rel, err := filepath.Rel(SkillsDir, path)
	return err == nil && !strings.HasPrefix(rel, "..")
}

func GetSkillName(path string) string {
	rel, err := filepath.Rel(SkillsDir, path)
	if err != nil {
		return "skills"
	}
	parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
	if parts[0] == "" || parts[0] == "." {
		return "skills"
	}
	return parts[0]
}
