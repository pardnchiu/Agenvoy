package app

import (
	"context"
	"log/slog"
	"os"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	sessionconfigbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type SendInput struct {
	Content      string
	SessionID    string
	Persist      bool
	Model        string
	Reasoning    string
	WorkDir      string
	Skill        string
	TaskHash     string
	Rule         string
	ExcludeTools []string
	PlainText    bool
	EnableEdit   *bool
	SkipSandbox  bool
}

func Send(ctx context.Context, in SendInput) (string, <-chan agentstypes.Event, error) {
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return "", nil, utilsmsg.New(utilsmsg.Invalid, "content", "required")
	}
	workDir, err := ResolveWorkDir(in.WorkDir)
	if err != nil {
		return "", nil, err
	}

	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		prefix := "temp-"
		if in.Persist {
			prefix = "chat-"
		}
		sessionID = prefix + go_pkg_utils.UUID()
	}
	if err := sessionconfigbot.Save(sessionID, "", "", false); err != nil {
		slog.Debug("/internal/session/config/bot: Save",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}

	taskHash := ""
	isRunning := exec.IsRunning(sessionID)
	if !isRunning {
		taskHash, isRunning = interactive.ClaimFollowUp(sessionID, strings.TrimSpace(in.TaskHash))
	}

	rule := in.Rule
	if in.PlainText {
		rule = strings.TrimSpace(rule + "\n\n" + configs.PlainTextRule)
	}
	data := exec.Prepare(exec.ExecuteMeta{
		PendingTask:       taskHash,
		Model:             in.Model,
		Reasoning:         in.Reasoning,
		WorkDir:           workDir,
		SkillName:         in.Skill,
		InputNoRecord:     content,
		Input:             content,
		SessionID:         sessionID,
		ExcludeTools:      in.ExcludeTools,
		ExtraSystemPrompt: rule,
	})

	if isRunning {
		exec.AppendSteer(sessionID, "", in.Content)
		return sessionID, nil, nil
	}

	execCtx := agentstypes.WithOrigin(ctx, "chat-")
	if in.EnableEdit != nil || in.SkipSandbox || !runtime.HasListener(execCtx, "chat-") {
		mode := agentstypes.AutoModeOn
		if in.EnableEdit != nil && *in.EnableEdit {
			mode |= agentstypes.AutoModeAllowEdit
		}
		if in.SkipSandbox {
			mode |= agentstypes.AutoModeSkipSandbox
		}
		execCtx = agentstypes.WithAutoMode(execCtx, mode)
		boundary.Revoke(sessionID)
	}
	events, _ := exec.Stream(execCtx, sessionID, 64, func(stream chan<- agentstypes.Event) error {
		if err := exec.Run(execCtx, data, stream, false); err != nil {
			stream <- agentstypes.ErrorEvent(err)
		}
		return nil
	})
	return sessionID, events, nil
}

func ResolveWorkDir(input string) (string, error) {
	home, _ := os.UserHomeDir()
	dir := strings.TrimSpace(input)
	if dir == "" {
		return home, nil
	}

	workDir, err := go_pkg_filesystem.AbsPath(home, dir, go_pkg_filesystem.AbsPathOption{})
	if err != nil {
		return "", utilsmsg.New(utilsmsg.Invalid, "work_dir", "%q cannot be resolved: %w", dir, err)
	}
	if !go_pkg_filesystem_reader.Exists(workDir) {
		return "", utilsmsg.New(utilsmsg.NotFound, "work_dir", "%q", dir)
	}
	if !go_pkg_filesystem_reader.IsDir(workDir) {
		return "", utilsmsg.New(utilsmsg.Invalid, "work_dir", "%q: not a directory", dir)
	}
	return workDir, nil
}
