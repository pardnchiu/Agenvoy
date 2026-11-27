package handler

import (
	"cmp"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/app"
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

		sessionID, events, err := app.Send(context.WithoutCancel(c.Request.Context()), app.SendInput{
			Content:      req.Content,
			SessionID:    req.SessionID,
			Persist:      req.Persist,
			Model:        req.Model,
			Reasoning:    req.Reasoning,
			WorkDir:      req.WorkDir,
			Skill:        req.Skill,
			TaskHash:     cmp.Or(req.TaskHash, req.PendingTask), // NOTE: PendingTask 棄用時要記得改
			Rule:         cmp.Or(req.Rule, req.SystemPrompt),    // NOTE: SystemPrompt 棄用時要記得改
			ExcludeTools: req.ExcludeTools,
			PlainText:    req.EnableMarkdown != nil && !*req.EnableMarkdown,
			EnableEdit:   req.EnableEdit,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if events == nil {
			c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
			return
		}

		if cmp.Or(req.Stream, req.SSE) { // NOTE: SSE 棄用時要記得改
			sendSSE(c, sessionID, req.Content, events)
		} else {
			sendResult(c, sessionID, req.Content, events)
		}
		drainEvents(events)
	}
}
