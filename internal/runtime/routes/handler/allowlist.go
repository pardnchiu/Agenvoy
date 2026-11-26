package handler

import (
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/agents"
	agentsexecallowskill "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/skill"
	agentsexecallowtool "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/tool"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

func allowSkillBlock(c *gin.Context) (gin.H, bool) {

	scanner := agents.Scanner()
	if scanner == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "skill scanner unavailable"})
		return nil, false
	}
	scanner.Scan()
	names := scanner.List()
	sort.Strings(names)

	allowed := agentsexecallowskill.Load()

	source := make(map[string]string, len(names))
	for _, name := range names {
		if sk := scanner.Lookup(name); sk != nil {
			source[name] = runtime.SkillSource(sk.AbsPath)
		}
	}
	return gin.H{"skills": names, "allowed": allowed, "source": source}, true
}

func allowToolBlock(c *gin.Context) gin.H {
	prefix := strings.TrimSpace(c.Query("prefix"))

	entries := make([]string, 0, 16)
	for entry := range agentsexecallowtool.Load() {
		if prefix == "" || strings.HasPrefix(entry, prefix) {
			entries = append(entries, entry)
		}
	}
	slices.Sort(entries)
	return gin.H{"entries": entries}
}

func GetAllowlist() gin.HandlerFunc {
	return func(c *gin.Context) {
		skill, ok := allowSkillBlock(c)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, gin.H{"skill": skill, "tool": allowToolBlock(c)})
	}
}

func SetAllowlist() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Skill *struct {
				Name string `json:"name"`
			} `json:"skill"`
			Tool *struct {
				Prefix  string   `json:"prefix"`
				Entries []string `json:"entries"`
			} `json:"tool"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.Skill == nil && body.Tool == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "skill or tool is required"})
			return
		}

		out := gin.H{"ok": true}

		if body.Tool != nil {
			prefix := strings.TrimSpace(body.Tool.Prefix)
			if prefix == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "tool.prefix is required"})
				return
			}
			entries := make([]string, 0, len(body.Tool.Entries))
			for _, entry := range body.Tool.Entries {
				entry = strings.TrimSpace(entry)
				if entry == "" {
					continue
				}
				if !strings.HasPrefix(entry, prefix) {
					c.JSON(http.StatusBadRequest, gin.H{"error": "every entry must start with the prefix: " + entry})
					return
				}
				entries = append(entries, entry)
			}
			if slices.Contains(entries, prefix+"*") {
				entries = []string{prefix + "*"}
			}
			if err := agentsexecallowtool.ToggleMCP(prefix, entries); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out["tool"] = gin.H{"entries": entries}
		}

		if body.Skill != nil {
			name := strings.TrimSpace(body.Skill.Name)
			if name == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "skill.name is required"})
				return
			}
			added, err := agentsexecallowskill.Toggle(name)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out["skill"] = gin.H{"added": added}
		}

		c.JSON(http.StatusOK, out)
	}
}
