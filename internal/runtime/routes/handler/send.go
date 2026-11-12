package handler

import (
	"cmp"
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	"github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	sessionconfigbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type Request struct {
	Content        string   `json:"content"`
	Stream         bool     `json:"stream,omitempty"`
	SessionID      string   `json:"session_id,omitempty"`
	Model          string   `json:"model,omitempty"`
	Reasoning      string   `json:"reasoning,omitempty"`
	ExcludeTools   []string `json:"exclude_tools,omitempty"`
	Persist        bool     `json:"persist,omitempty"`
	Rule           string   `json:"rule,omitempty"`
	WorkDir        string   `json:"work_dir,omitempty"`
	Skill          string   `json:"skill,omitempty"`
	TaskHash       string   `json:"task_hash,omitempty"`
	EnableEdit     *bool    `json:"enable_edit,omitempty"`     // NOTE: 改為預設唯讀模式
	EnableSandbox  *bool    `json:"enable_sandbox,omitempty"`  // NOTE: 增加關閉沙盒模式
	EnableMarkdown *bool    `json:"enable_markdown,omitempty"` // NOTE: 增加關閉 Markdown

	// WARN: 即將棄用
	SSE          bool   `json:"sse,omitempty"`           // NOTE: 改用 Stream
	SystemPrompt string `json:"system_prompt,omitempty"` // NOTE: 改用 Rule
	PendingTask  string `json:"pending_task,omitempty"`  // NOTE: 改用 TaskHash
}

func Send() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req Request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if strings.TrimSpace(req.Content) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "[Invalid] content: required"})
			return
		}

		workDir, err := resolveWorkDir(req.WorkDir)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		sessionID := strings.TrimSpace(req.SessionID)
		if sessionID == "" {
			prefix := "temp-"
			if req.Persist {
				prefix = "chat-"
			}
			sessionID = prefix + utils.UUID()
		}
		if err := sessionconfigbot.Save(sessionID, "", "", false); err != nil {
			slog.Debug("/internal/session/config/bot: Save",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}

		taskHash := ""
		isRunning := exec.IsRunning(sessionID)
		if !isRunning {
			// NOTE: 檢查 task hash 是否存在
			taskHash, isRunning = interactive.ClaimFollowUp(sessionID, strings.TrimSpace(cmp.Or(req.TaskHash, req.PendingTask))) // NOTE: PendingTask 棄用時要記得改
		}

		trimContent := strings.TrimSpace(req.Content)
		rule := cmp.Or(req.Rule, req.SystemPrompt) // NOTE: SystemPrompt 棄用時要記得改
		if req.EnableMarkdown != nil && !*req.EnableMarkdown {
			rule = strings.TrimSpace(rule + "\n\n" + configs.PlainTextRule)
		}
		data := exec.Prepare(exec.ExecuteMeta{
			PendingTask:       taskHash,
			Model:             req.Model,
			Reasoning:         req.Reasoning,
			WorkDir:           workDir,
			SkillName:         req.Skill,
			InputNoRecord:     trimContent,
			Input:             trimContent,
			SessionID:         sessionID,
			ExcludeTools:      req.ExcludeTools,
			ExtraSystemPrompt: rule,
		})

		// NOTE: 如果已經在執行中，則直接將內容加入到執行中
		if isRunning {
			exec.AppendSteer(sessionID, "", req.Content)
			c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
			return
		}

		execCtx := agentstypes.WithOrigin(context.WithoutCancel(c.Request.Context()), "chat-")
		if req.EnableEdit != nil || req.EnableSandbox != nil || !runtime.HasListener(execCtx, "chat-") {
			mode := agentstypes.AutoModeOn
			if req.EnableEdit != nil && *req.EnableEdit {
				mode |= agentstypes.AutoModeAllowEdit
			}
			if req.EnableSandbox != nil && !*req.EnableSandbox {
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

		if cmp.Or(req.Stream, req.SSE) { // NOTE: SSE 棄用時要記得改
			sendSSE(c, sessionID, req.Content, events)
		} else {
			sendResult(c, sessionID, req.Content, events)
		}
		drainEvents(events)
	}
}

func resolveWorkDir(input string) (string, error) {
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
