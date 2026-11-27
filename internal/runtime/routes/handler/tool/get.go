package tool

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/configs"
	agentsexecallowtool "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/tool"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	runtimetooladapter "github.com/pardnchiu/agenvoy/internal/runtime/toolAdapter"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Get() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.Trim(c.Param("name"), "/")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": utilsmsg.New(utilsmsg.Invalid, "name", "required").Error()})
			return
		}

		translator := runtimetooladapter.New(configs.SCRIPT_TOOL_PREFIX)
		translator.Scan(filesystem.ScriptToolsDir)
		doc, dir, ok := translator.Lookup(name)
		if !ok || !slices.Contains(toolNames(translator), name) {
			c.JSON(http.StatusNotFound, gin.H{"error": utilsmsg.New(utilsmsg.NotFound, "script tool", "%q", name).Error()})
			return
		}
		alwaysAllow := slices.Contains(translator.AlwaysAllowNames(), name)

		c.JSON(http.StatusOK, gin.H{
			"name":         name,
			"description":  doc.Description,
			"path":         dir,
			"allowed":      agentsexecallowtool.Read()[name] || alwaysAllow,
			"always_allow": alwaysAllow,
			"files":        toolFiles(dir),
		})
	}
}

func List() gin.HandlerFunc {
	return func(c *gin.Context) {
		translator := runtimetooladapter.New(configs.SCRIPT_TOOL_PREFIX)
		translator.Scan(filesystem.ScriptToolsDir)
		alwaysAllowNames := translator.AlwaysAllowNames()
		allowed := agentsexecallowtool.Read()

		list := make([]gin.H, 0)
		for _, name := range toolNames(translator) {
			doc, dir, ok := translator.Lookup(name)
			if !ok {
				continue
			}

			files := toolFiles(dir)
			paths := make([]string, 0, len(files))
			for _, one := range files {
				paths = append(paths, one.Path)
			}
			list = append(list, gin.H{
				"name":         name,
				"description":  doc.Description,
				"allowed":      allowed[name] || slices.Contains(alwaysAllowNames, name),
				"always_allow": slices.Contains(alwaysAllowNames, name),
				"files":        paths,
			})
		}
		c.JSON(http.StatusOK, gin.H{"tools": list, "allow_path": filesystem.AllowToolPath})
	}
}
