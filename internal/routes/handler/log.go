package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

const (
	logTailLines      = 100
	logPollInterval   = time.Second
	logHeartbeatTicks = 15
)

func StreamSessionLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := strings.TrimSpace(c.Param("session_id"))
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
			return
		}

		dir := filepath.Join(filesystem.SessionsDir, sessionID)
		if _, err := os.Stat(dir); err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		h := c.Writer.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Flush()

		ctx := c.Request.Context()
		ticker := time.NewTicker(logPollInterval)
		defer ticker.Stop()

		var (
			lastLine   string
			quietTicks int
		)

		emit := func() bool {
			lines := sessionManager.GeadRecord(sessionID, logTailLines)

			startIdx := 0
			if lastLine != "" {
				for i := len(lines) - 1; i >= 0; i-- {
					if lines[i] == lastLine {
						startIdx = i + 1
						break
					}
				}
			}

			payload := lines[startIdx:]
			if len(payload) == 0 {
				quietTicks++
				if quietTicks >= logHeartbeatTicks {
					quietTicks = 0
					if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
						return false
					}
					c.Writer.Flush()
				}
				return true
			}

			quietTicks = 0
			for _, line := range payload {
				if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", line); err != nil {
					return false
				}
			}
			c.Writer.Flush()
			lastLine = lines[len(lines)-1]
			return true
		}

		if !emit() {
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !emit() {
					return
				}
			}
		}
	}
}
