package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/audio"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	imagetool "github.com/pardnchiu/agenvoy/internal/tools/external/image"
)

func DeleteKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Query("key")
		if key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
			return
		}
		if err := keychain.Delete(key); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := config.DeleteKey(key); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		imagetool.Prune(c.Request.Context())
		audio.Prune(c.Request.Context(), audio.STT)
		audio.Prune(c.Request.Context(), audio.TTS)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
