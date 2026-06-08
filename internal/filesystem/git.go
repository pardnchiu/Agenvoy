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

type GitTarget int

const (
	GitSkills GitTarget = iota
	GitTools
)

type gitConfig struct {
	dir           func() string
	gitignorePath func() string
	ignoreEntries []string
	fallbackName  string
}

var gitConfigs = map[GitTarget]gitConfig{
	GitSkills: {
		dir:           func() string { return SkillsDir },
		gitignorePath: func() string { return SkillGitignorePath },
		ignoreEntries: []string{".system/", ".Trash/"},
		fallbackName:  "skills",
	},
	GitTools: {
		dir:           func() string { return ToolsDir },
		gitignorePath: func() string { return ToolGitignorePath },
		ignoreEntries: []string{".system/", ".extension/", ".Trash/"},
		fallbackName:  "tools",
	},
}

func GitCheckInit(ctx context.Context, t GitTarget) error {
	cfg := gitConfigs[t]
	dir := cfg.dir()

	if err := gitCheckIgnore(cfg); err != nil {
		return err
	}

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

func gitCheckIgnore(cfg gitConfig) error {
	dir := cfg.dir()
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir [%s]: %w", dir, err)
	}

	ignorePath := cfg.gitignorePath()
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
	for _, entry := range cfg.ignoreEntries {
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

func GitCommit(ctx context.Context, t GitTarget, act, name string) error {
	dir := gitConfigs[t].dir()
	now := time.Now().Format("20060102")
	message := fmt.Sprintf("%s_%s_%s", act, name, now)

	cmd := exec.CommandContext(ctx, "git", "add", "-A")
	cmd.Dir = dir
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput (git add -A): %w", err)
	}

	status := exec.CommandContext(ctx, "git", "status", "--porcelain")
	status.Dir = dir
	output, err := status.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput (git status --porcelain): %w", err)
	}
	if strings.TrimSpace(string(output)) == "" {
		return nil
	}

	cmd = exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = dir
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exec.Cmd.CombinedOutput (git commit -m): %w", err)
	}
	return nil
}

func GitAutoCommit(ctx context.Context, t GitTarget, act, name string) {
	if err := GitCheckInit(ctx, t); err != nil {
		return
	}
	if err := GitCommit(ctx, t, act, name); err != nil {
		slog.Warn("GitCommit",
			slog.String("target", gitConfigs[t].fallbackName),
			slog.String("action", act),
			slog.String("name", name),
			slog.String("error", err.Error()))
	}
}

func GitAutoCommitByPath(ctx context.Context, t GitTarget, path string, isNew bool) {
	cfg := gitConfigs[t]
	dir := cfg.dir()
	rel, err := filepath.Rel(dir, path)
	if !(err == nil && !strings.HasPrefix(rel, "..")) {
		return
	}

	act := "update"
	if isNew {
		act = "add"
	}
	GitAutoCommit(ctx, t, act, gitNameFromPath(cfg, path))
}

func gitNameFromPath(cfg gitConfig, path string) string {
	rel, err := filepath.Rel(cfg.dir(), path)
	if err != nil {
		return cfg.fallbackName
	}

	parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
	if parts[0] == "" || parts[0] == "." {
		return cfg.fallbackName
	}
	return parts[0]
}

func GitLog(ctx context.Context, t GitTarget, limit int) (string, error) {
	if limit <= 0 {
		limit = 20
	}

	cmd := exec.CommandContext(ctx, "git", "log", "--oneline", fmt.Sprintf("-n%d", limit))
	cmd.Dir = gitConfigs[t].dir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("exec.Cmd.CombinedOutput (git log --oneline): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func GitRollback(ctx context.Context, t GitTarget, commit string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "reset", "--hard", commit)
	cmd.Dir = gitConfigs[t].dir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("exec.Cmd.CombinedOutput (git reset --hard): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
