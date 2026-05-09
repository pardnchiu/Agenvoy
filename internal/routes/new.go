package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/routes/handler"
)

func New() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/v1/send", handler.Send())
	r.POST("/v1/key", handler.SaveKey())
	r.GET("/v1/key", handler.GetKey())
	r.GET("/v1/tools", handler.ListTools())
	r.POST("/v1/tool/:tool_name", handler.CallTool())
	r.GET("/v1/session/:session_id/status", handler.GetSessionStatus())
	r.GET("/v1/session/:session_id/log", handler.StreamSessionLog())

	return r
}
