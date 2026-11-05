package handler

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	"github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionconfigbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type Request struct {
	Content      string   `json:"content"`
	SSE          bool     `json:"sse"`
	SessionID    string   `json:"session_id"`
	Model        string   `json:"model,omitempty"`
	ExcludeTools []string `json:"exclude_tools,omitempty"`
	Persist      bool     `json:"persist,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	WorkDir      string   `json:"work_dir,omitempty"`
	Skill        string   `json:"skill,omitempty"`
	PendingTask  string   `json:"pending_task,omitempty"`
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

		sessionID := req.SessionID
		if sessionID == "" {
			prefix := "temp-"
			if req.Persist {
				prefix = "chat-"
			}
			sessionID = prefix + utils.UUID()
		}

		if err := sessionconfigbot.Save(sessionID, "", "", false); err != nil {
			slog.Debug("sessionBot Save",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}

		steer := exec.IsRunning(sessionID)
		pendingTask := ""
		if !steer {
			pendingTask, steer = interactive.ClaimFollowUp(sessionID, strings.TrimSpace(req.PendingTask))
		}

		trimContent := strings.TrimSpace(req.Content)
		data := exec.Prepare(exec.ExecuteMeta{
			PendingTask:       pendingTask,
			Model:             req.Model,
			WorkDir:           workDir,
			SkillName:         req.Skill,
			Task:              trimContent,
			Input:             trimContent,
			SessionID:         sessionID,
			ExcludeTools:      req.ExcludeTools,
			ExtraSystemPrompt: req.SystemPrompt,
		})

		if steer {
			exec.AppendSteer(sessionID, "", req.Content)
			c.JSON(http.StatusOK, gin.H{
				"session_id": sessionID,
				"steer":      true,
			})
			return
		}

		execCtx := agentstypes.WithOrigin(context.WithoutCancel(c.Request.Context()), "chat-")
		events, _ := exec.Stream(execCtx, sessionID, 64, func(stream chan<- agentstypes.Event) error {
			if err := exec.Run(execCtx, data, stream, false); err != nil {
				stream <- agentstypes.ErrorEvent(err)
			}
			return nil
		})

		if req.SSE {
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

	resolved, err := go_pkg_filesystem.AbsPath(home, dir, go_pkg_filesystem.AbsPathOption{})
	if err != nil {
		return "", utilsmsg.New(utilsmsg.Invalid, "work_dir", "%q cannot be resolved: %w", dir, err)
	}
	if !go_pkg_filesystem_reader.Exists(resolved) {
		return "", utilsmsg.New(utilsmsg.NotFound, "work_dir", "%q", dir)
	}
	if !go_pkg_filesystem_reader.IsDir(resolved) {
		return "", utilsmsg.New(utilsmsg.Invalid, "work_dir", "%q: not a directory", dir)
	}
	return resolved, nil
}
