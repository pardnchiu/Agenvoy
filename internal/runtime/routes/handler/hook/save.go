package hook

import (
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	agentsexechook "github.com/pardnchiu/agenvoy/internal/agents/exec/hook"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Save() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name    string `json:"name"`
			Ext     string `json:"ext"`
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": utilsmsg.Wrap(utilsmsg.Invalid, "gin: ShouldBindJSON", "", err).Error()})
			return
		}

		path, err := target(body.Name, body.Ext)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if strings.TrimSpace(body.Content) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": utilsmsg.New(utilsmsg.Invalid, "content", "required").Error()})
			return
		}
		if err := go_pkg_filesystem.WriteFile(path, body.Content, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: WriteFile", path, err).Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": strings.TrimSpace(body.Name), "path": path})
	}
}

func target(name, ext string) (string, error) {
	name = strings.TrimSpace(name)
	if !slices.Contains(agentsexechook.Names(), name) {
		return "", utilsmsg.New(utilsmsg.Invalid, "name", "unknown hook %q; available: %s", name, strings.Join(agentsexechook.Names(), ", "))
	}

	ext = strings.TrimSpace(ext)
	if !slices.Contains(hookExts, ext) {
		return "", utilsmsg.New(utilsmsg.Invalid, "ext", "%q; available: py, js", ext)
	}
	return filepath.Join(filesystem.HooksDir, name+"."+ext), nil
}
