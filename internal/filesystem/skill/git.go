package skill

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"os/exec"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

var skillGitignoreEntries = []string{".system/", ".Trash/"}

func CheckGit(ctx context.Context) error {
	if err := checkGitignore(); err != nil {
		return err
	}

	dir := filesystem.SkillsDir
	if go_pkg_filesystem_reader.IsDir(filepath.Join(dir, ".git")) {
		return nil
	}

	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = dir
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput(git init): %w", err)
	}

	return nil
}

func checkGitignore() error {
	dir := filesystem.SkillsDir
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir [%s]: %w", dir, err)
	}

	ignorePath := filesystem.SkillGitignorePath
	var content string
	if go_pkg_filesystem_reader.Exists(ignorePath) {
		str, err := go_pkg_filesystem.ReadText(ignorePath)
		if err != nil {
			return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadText [%s]: %w", ignorePath, err)
		}
		content = str
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

	if err := go_pkg_filesystem.WriteFile(ignorePath, content, 0644); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteFile [%s]: %w", ignorePath, err)
	}
	return nil
}

func Commit(ctx context.Context, act, skillName string) error {
	now := time.Now().Format("20060102")
	message := fmt.Sprintf("%s_%s_%s", act, skillName, now)

	cmd := exec.CommandContext(ctx, "git", "add", "-A")
	cmd.Dir = filesystem.SkillsDir
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput (git add -A): %w", err)
	}

	status := exec.CommandContext(ctx, "git", "status", "--porcelain")
	status.Dir = filesystem.SkillsDir
	output, err := status.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput (git status --porcelain): %w", err)
	}
	if strings.TrimSpace(string(output)) == "" {
		return nil
	}

	cmd = exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = filesystem.SkillsDir
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput (git commit -m): %w", err)
	}
	return nil
}

func AutoCommit(ctx context.Context, act, name string) {
	if err := CheckGit(ctx); err != nil {
		return
	}
	if err := Commit(ctx, act, name); err != nil {
		slog.Warn("Commit",
			slog.String("action", act),
			slog.String("name", name),
			slog.String("error", err.Error()))
	}
}

func AutoCommitByPath(ctx context.Context, path string, isNew bool) {
	rel, err := filepath.Rel(filesystem.SkillsDir, path)
	if !(err == nil && !strings.HasPrefix(rel, "..")) {
		return
	}

	act := "update"
	if isNew {
		act = "add"
	}
	AutoCommit(ctx, act, getSkillName(path))
}

func getSkillName(path string) string {
	rel, err := filepath.Rel(filesystem.SkillsDir, path)
	if err != nil {
		return "skills"
	}

	parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
	if parts[0] == "" || parts[0] == "." {
		return "skills"
	}
	return parts[0]
}

func GetGitLog(ctx context.Context, limit int) (string, error) {
	if limit <= 0 {
		limit = 20
	}

	cmd := exec.CommandContext(ctx, "git", "log", "--oneline", fmt.Sprintf("-n%d", limit))
	cmd.Dir = filesystem.SkillsDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("exec.Cmd.CombinedOutput (git log --oneline): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func Rollback(ctx context.Context, commit string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "reset", "--hard", commit)
	cmd.Dir = filesystem.SkillsDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("exec.Cmd.CombinedOutput (git reset --hard): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
