package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/host"
	"github.com/pardnchiu/agenvoy/internal/tools"
)

func ListTools() gin.HandlerFunc {
	return func(c *gin.Context) {
		workDir, _ := os.UserHomeDir()
		executor, err := tools.NewExecutor(workDir, "api-"+utils.UUID(), host.Scanner())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		type toolItem struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		}

		items := make([]toolItem, 0, len(executor.Tools))
		for _, t := range executor.Tools {
			items = append(items, toolItem{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			})
		}

		c.JSON(http.StatusOK, gin.H{"tools": items})
	}
}

func CallTool() gin.HandlerFunc {
	return func(c *gin.Context) {
		toolName := c.Param("tool_name")

		var args json.RawMessage
		if err := c.ShouldBindJSON(&args); err != nil {
			args = json.RawMessage("{}")
		}

		workDir, _ := os.UserHomeDir()
		executor, err := tools.NewExecutor(workDir, "api-"+utils.UUID(), host.Scanner())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		found := false
		for _, t := range executor.Tools {
			if t.Function.Name == toolName {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found: " + toolName})
			return
		}

		result, err := tools.Execute(context.Background(), executor, toolName, args)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"result": result})
	}
}
