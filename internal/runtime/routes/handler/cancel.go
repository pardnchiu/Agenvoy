package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	sessionlog "github.com/pardnchiu/agenvoy/internal/session/log"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

func CancelSessionTask() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := strings.TrimSpace(c.Param("session_id"))
		taskHash := strings.TrimSpace(c.Param("task_hash"))
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
			return
		}

		window := interactive.LoadPendingWindow(sessionID, taskHash)
		if taskHash != "" && taskHash != "current" {
			interactive.CancelTask(sessionID, taskHash)
		}
		if taskHash != "" && taskHash != "current" && exec.Cancel(taskHash, runtime.ErrUserCanceled) {
			c.JSON(http.StatusOK, gin.H{"ok": true, "cancelled": true})
			return
		}

		event := agenttypes.Event{Type: agenttypes.EventCanceled, TaskHash: taskHash, WindowHash: window}
		sessionlog.Record(sessionID, event)
		pubsub.Pub(sessionID, event)
		c.JSON(http.StatusOK, gin.H{"ok": true, "cancelled": false, "stale": true})
	}
}
