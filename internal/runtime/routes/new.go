package routes

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/runtime/routes/handler"
	completionsHandler "github.com/pardnchiu/agenvoy/internal/runtime/routes/handler/chatCompletions"
)

func New() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors())

	r.POST("/v1/chat/completions", completionsHandler.ChatCompletions())
	r.POST("/v1/send", handler.Send())

	r.GET("/v1/tools", handler.ListTools())
	r.POST("/v1/tool/:tool_name", handler.CallTool())
	r.GET("/v1/sessions", handler.ListSessions())
	r.GET("/v1/session/:session_id/status", handler.GetSessionStatus())
	r.GET("/v1/session/:session_id/log", handler.StreamSessionLog())
	r.GET("/v1/log", handler.StreamMultiLog())
	r.POST("/v1/session/:session_id/event", localhostOnly(), handler.PublishSessionEvent())
	r.GET("/v1/session/:session_id/pending", handler.ListSessionPending())
	r.GET("/v1/session/:session_id/pending/:task_hash/questions", handler.GetSessionPendingQuestions())
	r.POST("/v1/session/:session_id/pending/:task_hash/resume", handler.ResumeSessionPending())
	r.GET("/v1/key", localhostOnly(), handler.GetKey())

	return r
}

var allowedOrigins = map[string]bool{
	"https://web.agenvoy.com": true,
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowedOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			c.Header("Access-Control-Allow-Private-Network", "true")
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}

func localhostOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			host = c.Request.RemoteAddr
		}
		switch host {
		case "127.0.0.1", "::1":
			c.Next()
		default:
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"message": "localhost only", "type": "forbidden"}})
		}
	}
}
