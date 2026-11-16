package handler

import (
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
