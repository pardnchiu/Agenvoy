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

func Toggle() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": utilsmsg.Wrap(utilsmsg.Invalid, "gin: ShouldBindJSON", "", err).Error()})
			return
		}

		name := strings.TrimSpace(body.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": utilsmsg.New(utilsmsg.Invalid, "name", "required").Error()})
			return
		}

		translator := runtimetooladapter.New(configs.SCRIPT_TOOL_PREFIX)
		translator.Scan(filesystem.ScriptToolsDir)
		if !slices.Contains(toolNames(translator), name) {
			c.JSON(http.StatusNotFound, gin.H{"error": utilsmsg.New(utilsmsg.NotFound, "script tool", "%q", name).Error()})
			return
		}
		if slices.Contains(translator.AlwaysAllowNames(), name) {
			c.JSON(http.StatusConflict, gin.H{"error": utilsmsg.New(utilsmsg.Conflict, "script tool "+name, "always_allow is set in its tool.json; edit that file to change it").Error()})
			return
		}

		added, err := agentsexecallowtool.Toggle(name, "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "allowed": added})
	}
}
