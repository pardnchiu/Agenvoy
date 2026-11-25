package hook

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name string `json:"name"`
			Ext  string `json:"ext"`
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
		if !go_pkg_filesystem_reader.IsFile(path) {
			c.JSON(http.StatusNotFound, gin.H{"error": utilsmsg.New(utilsmsg.NotFound, "hook", "%s", path).Error()})
			return
		}

		trashPath, err := filesystem.TrashDir(path, filesystem.HookTrashDir, filepath.Base(path))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		_ = historystore.RecordDelete(context.Background(), path, trashPath, historystore.Meta{Tool: "hook"})
		c.JSON(http.StatusOK, gin.H{"trashed": path})
	}
}
