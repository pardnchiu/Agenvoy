package filesystem

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"os/exec"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

var skillGitignoreEntries = []string{".system/", ".Trash/"}

func CheckSkillGitDir(ctx context.Context) error {
	if err := go_pkg_filesystem.CheckDir(SkillsDir, true); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir(%s): %w", SkillsDir, err)
	}

	if err := checkSkillGitignore(); err != nil {
		return err
	}

	if go_pkg_filesystem_reader.Exists(SkillGitDir) {
		return nil
	}

	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = SkillsDir
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput(git init): %w", err)
	}

	return nil
}

func checkSkillGitignore() error {
	var content string
	if go_pkg_filesystem_reader.Exists(SkillGitignorePath) {
		body, err := go_pkg_filesystem.ReadText(SkillGitignorePath)
		if err != nil {
			return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadText(%s): %w", SkillGitignorePath, err)
		}
		content = body
	}

	have := make(map[string]bool)
	for line := range strings.SplitSeq(content, "\n") {
		have[strings.TrimSpace(line)] = true
	}

	var added bool
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	for _, entry := range skillGitignoreEntries {
		if have[entry] {
			continue
		}
		content += entry + "\n"
		added = true
	}
	if !added {
		return nil
	}

	if err := go_pkg_filesystem.WriteFile(SkillGitignorePath, content, 0644); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteFile(%s): %w", SkillGitignorePath, err)
	}
	return nil
}

func CommitSkillDir(ctx context.Context, act, skillName string) error {
	date := time.Now().Format("20060102")
	message := fmt.Sprintf("%s_%s_%s", act, skillName, date)

	cmd := exec.CommandContext(ctx, "git", "add", "-A")
	cmd.Dir = SkillsDir
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput (git add -A): %w", err)
	}

	status := exec.CommandContext(ctx, "git", "status", "--porcelain")
	status.Dir = SkillsDir
	output, err := status.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput (git status --porcelain): %w", err)
	}
	if strings.TrimSpace(string(output)) == "" {
		return nil
	}

	cmd = exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = SkillsDir
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput (git commit -m): %w", err)
	}
	return nil
}

func GetSkillDirGitLog(ctx context.Context, limit int) (string, error) {
	if limit <= 0 {
		limit = 20
	}

	cmd := exec.CommandContext(ctx, "git", "log", "--oneline", fmt.Sprintf("-n%d", limit))
	cmd.Dir = SkillsDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("exec.Cmd.CombinedOutput (git log --oneline): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func RollbackSkillDir(ctx context.Context, commit string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "reset", "--hard", commit)
	cmd.Dir = SkillsDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("exec.Cmd.CombinedOutput (git reset --hard): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
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

func RunCommitSkillDir(ctx context.Context, path string, isNew bool) {
	rel, err := filepath.Rel(SkillsDir, path)
	if !(err == nil && !strings.HasPrefix(rel, "..")) {
		return
	}
	if err := CheckSkillGitDir(ctx); err != nil {
		return
	}

	act := "update"
	if isNew {
		act = "add"
	}
	name := GetSkillName(path)
	if err := CommitSkillDir(ctx, act, name); err != nil {
		slog.Warn("CommitSkillDir",
			slog.String("action", act),
			slog.String("name", name),
			slog.String("error", err.Error()))
	}
}

func RunTrashCommitSkillDir(ctx context.Context, name string) {
	if err := CheckSkillGitDir(ctx); err != nil {
		return
	}
	if err := CommitSkillDir(ctx, "trash", name); err != nil {
		slog.Warn("CommitSkillDir",
			slog.String("action", "trash"),
			slog.String("name", name),
			slog.String("error", err.Error()))
	}
}
