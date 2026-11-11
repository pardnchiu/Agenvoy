package handler

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools"
)

func resolveFilePath(raw string) (string, int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", http.StatusBadRequest, os.ErrInvalid
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", http.StatusInternalServerError, err
	}
	absPath, err := go_pkg_filesystem.AbsPath(home, raw, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
	if err != nil {
		return "", http.StatusForbidden, err
	}
	if filesystem.IsSensitivePath(absPath) {
		return "", http.StatusForbidden, os.ErrPermission
	}
	return absPath, 0, nil
}

func fileSHA256(absPath string) (string, error) {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func GetFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		absPath, code, err := resolveFilePath(c.Query("path"))
		if err != nil {
			c.JSON(code, gin.H{"error": err.Error()})
			return
		}

		info, err := os.Stat(absPath)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if info.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path is a directory"})
			return
		}

		if sum, err := fileSHA256(absPath); err == nil {
			c.Header("X-Content-SHA256", sum)
		}
		c.File(absPath)
	}
}

func OpenFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		absPath, code, err := resolveFilePath(c.Query("path"))
		if err != nil {
			c.JSON(code, gin.H{"error": err.Error()})
			return
		}
		result, err := tools.OpenFile(c.Request.Context(), "", "", absPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"result": result})
	}
}

func PutFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Path          string `json:"path"`
			Content       string `json:"content"`
			ContentBase64 string `json:"content_base64"`
			BaseSHA256    string `json:"base_sha256"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		payload := req.Content
		if req.ContentBase64 != "" {
			raw, decErr := base64.StdEncoding.DecodeString(req.ContentBase64)
			if decErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid content_base64: " + decErr.Error()})
				return
			}
			payload = string(raw)
		}

		absPath, code, err := resolveFilePath(req.Path)
		if err != nil {
			c.JSON(code, gin.H{"error": err.Error()})
			return
		}

		current, statErr := fileSHA256(absPath)
		switch {
		case statErr == nil:
			if req.BaseSHA256 != current {
				c.JSON(http.StatusConflict, gin.H{
					"error":          "file changed on disk since it was opened",
					"current_sha256": current,
				})
				return
			}
		case os.IsNotExist(statErr):
			if req.BaseSHA256 != "" {
				c.JSON(http.StatusConflict, gin.H{"error": "file no longer exists"})
				return
			}
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": statErr.Error()})
			return
		}

		if err := go_pkg_filesystem.WriteFile(absPath, payload, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		sum, err := fileSHA256(absPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"sha256": sum, "path": absPath})
	}
}
