package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	configStatus "github.com/pardnchiu/agenvoy/internal/session/config/status"
)

type SessionInfo struct {
	ID      string              `json:"id"`
	Name    string              `json:"name"`
	State   string              `json:"state"`
	Model   string              `json:"model"`
	Active  []configStatus.Task `json:"active"`
	EndedAt string              `json:"ended_at"`
}

func ListSessions() gin.HandlerFunc {
	return func(c *gin.Context) {
		filter := c.DefaultQuery("filter", "all")

		dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"sessions": []SessionInfo{}})
			return
		}

		list := make([]SessionInfo, 0, len(dirs))
		for _, dir := range dirs {
			sid := dir.Name
			if strings.HasPrefix(sid, ".") || sid == "jarvis" {
				continue
			}

			switch filter {
			case "active":
				status := configStatus.Get(sid)
				if status.State != configStatus.StatusOnline {
					continue
				}
			case "permanent":
				if strings.HasPrefix(sid, "temp-") {
					continue
				}
			case "temporary":
				if !strings.HasPrefix(sid, "temp-") {
					continue
				}
			}

			status := configStatus.Get(sid)
			name, _ := configBot.Get(sid)
			model, _ := configBot.GetModel(sid)

			if status.Active == nil {
				status.Active = []configStatus.Task{}
			}

			list = append(list, SessionInfo{
				ID:      sid,
				Name:    name,
				State:   status.State,
				Model:   model,
				Active:  status.Active,
				EndedAt: status.EndedAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{"sessions": list})
	}
}
