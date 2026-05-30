package handler

import (
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionStatus "github.com/pardnchiu/agenvoy/internal/session/status"
)

type SessionStatus struct {
	State   string               `json:"state"`
	Active  []sessionStatus.Task `json:"active"`
	EndedAt string               `json:"ended_at"`
	Limit   int                  `json:"limit"`
	Usage   float64              `json:"usage"`
}

func GetSessionStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := strings.TrimSpace(c.Param("session_id"))
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
			return
		}

		dir := filesystem.SessionDir(sessionID)
		if !go_pkg_filesystem_reader.Exists(dir) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}

		status := sessionStatus.Get(sessionID)
		limit := filesystem.MaxSessionTasks
		usage := 0.0
		if limit > 0 {
			usage = math.Round(float64(len(status.Active))/float64(limit)*10000) / 100
		}
		if status.Active == nil {
			status.Active = []sessionStatus.Task{}
		}

		c.JSON(http.StatusOK, SessionStatus{
			State:   status.State,
			Active:  status.Active,
			EndedAt: status.EndedAt,
			Limit:   limit,
			Usage:   usage,
		})
	}
}
