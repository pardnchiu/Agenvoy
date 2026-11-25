package hook

import (
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	agentsexechook "github.com/pardnchiu/agenvoy/internal/agents/exec/hook"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var hookExts = []string{"py", "js"}

type file struct {
	Ext     string `json:"ext"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Primary bool   `json:"primary"`
	Content string `json:"content,omitempty"`
}

func Get() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.Trim(c.Param("name"), "/")
		if !slices.Contains(agentsexechook.Names(), name) {
			c.JSON(http.StatusNotFound, gin.H{"error": utilsmsg.New(utilsmsg.NotFound, "hook", "%q", name).Error()})
			return
		}
		label := agentsexechook.State(slices.Index(agentsexechook.Names(), name)).String()
		c.JSON(http.StatusOK, gin.H{"name": name, "label": label, "files": files(name, true)})
	}
}

func List() gin.HandlerFunc {
	return func(c *gin.Context) {
		list := make([]gin.H, 0, len(agentsexechook.Names()))
		for i, name := range agentsexechook.Names() {
			list = append(list, gin.H{"name": name, "label": agentsexechook.State(i).String(), "files": files(name, false)})
		}
		c.JSON(http.StatusOK, gin.H{"hooks": list})
	}
}

func files(name string, withContent bool) []file {
	files := make([]file, 0, len(hookExts))
	primary := ""
	for _, ext := range hookExts {
		path := filepath.Join(filesystem.HooksDir, name+"."+ext)
		one := file{Ext: ext, Path: path, Exists: go_pkg_filesystem_reader.IsFile(path)}
		if one.Exists && primary == "" {
			primary = ext
			one.Primary = true
		}
		if withContent {
			source := path
			if !one.Exists {
				source = filepath.Join(filesystem.HooksDir, "demo", name+"."+ext)
			}
			if text, err := go_pkg_filesystem.ReadText(source); err == nil {
				one.Content = text
			}
		}
		files = append(files, one)
	}
	return files
}
