package handler

import (
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

type sessionStatus struct {
	State   string                `json:"state"`
	Active  []sessionManager.Task `json:"active"`
	EndedAt string                `json:"ended_at"`
	Limit   int                   `json:"limit"`
	Usage   float64               `json:"usage"`
}

func GetSessionStatus() gin.HandlerFunc {
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

		status := sessionManager.ReadStatus(sessionID)
		limit := sessionManager.MaxConcurrentPerSession
		usage := 0.0
		if limit > 0 {
			usage = math.Round(float64(len(status.Active))/float64(limit)*10000) / 100
		}
		if status.Active == nil {
			status.Active = []sessionManager.Task{}
		}

		c.JSON(http.StatusOK, sessionStatus{
			State:   status.State,
			Active:  status.Active,
			EndedAt: status.EndedAt,
			Limit:   limit,
			Usage:   usage,
		})
	}
}
