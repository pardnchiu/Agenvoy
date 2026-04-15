package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/go-utils/filesystem/keychain"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

type keyStoreRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func SaveKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req keyStoreRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
			return
		}
		if req.Value == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "value is required"})
			return
		}

		if err := keychain.Set(req.Key, req.Value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := sessionManager.SaveKey(req.Key); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func GetKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Query("key")
		if key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
			return
		}
		if !sessionManager.IsKeyExist(key) {
			c.JSON(http.StatusForbidden, gin.H{"error": "key not registered"})
			return
		}

		value := keychain.Get(key)
		if value == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"key": key, "value": value})
	}
}
