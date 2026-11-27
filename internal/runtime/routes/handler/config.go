package handler

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/agenvoy/internal/startup"
)

const maxReplyLang = 64

func GetConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Param("target") {
		case "startup":
			c.JSON(http.StatusOK, gin.H{"enabled": startup.State(), "installed": startup.Enabled()})
		case "system":
			c.JSON(http.StatusOK, gin.H{
				"reply_lang": filesystem.GetReplyLang(),
				"languages":  filesystem.ReplyLangOptions(),
			})
		case "reply_lang":
			c.JSON(http.StatusOK, gin.H{
				"reply_lang": filesystem.GetReplyLang(),
				"name":       filesystem.ReplyLangName(filesystem.GetReplyLang()),
				"directive":  filesystem.ReplyLangDirective(),
			})
		case "output_dir":
			c.JSON(http.StatusOK, gin.H{
				"output_dir": filesystem.ConfigOutputDir,
				"resolved":   filesystem.OutputDir(),
			})
		case "official_guide":
			cfg, err := config.Load()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"enabled": !cfg.OfficialGuideOff})
		case "security":
			cfg, err := config.Load()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"command_network": !cfg.CommandNetOff,
				"hook_network":    !cfg.HookNetOff,
				"lan_access":      cfg.LANAccess,
			})
		case "skill_source":
			cfg, err := config.Load()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			disabled := cfg.SkillSourceOff
			if disabled == nil {
				disabled = []string{}
			}
			c.JSON(http.StatusOK, gin.H{"sources": runtime.SkillSources, "disabled": disabled})
		default:
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown config target"})
		}
	}
}

func SetConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Param("target") {
		case "startup":
			setStartup(c)
		case "system":
			setSystemConfig(c)
		case "output_dir":
			setOutputDir(c)
		case "official_guide":
			setOfficialGuide(c)
		case "security":
			setSecurity(c)
		case "skill_source":
			setSkillSource(c)
		default:
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown config target"})
		}
	}
}

func setStartup(c *gin.Context) {
	var body struct {
		Enable *bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Enable == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enable is required"})
		return
	}

	var (
		detail string
		err    error
	)
	if *body.Enable {
		detail, err = startup.Enable()
	} else {
		detail, err = startup.Disable()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": startup.State(), "installed": startup.Enabled(), "detail": detail})
}

func setSystemConfig(c *gin.Context) {
	var body struct {
		ReplyLang *string `json:"reply_lang"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.ReplyLang == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply_lang is required"})
		return
	}

	lang := filesystem.CanonicalReplyLang(*body.ReplyLang)
	if len(lang) > maxReplyLang {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply_lang is too long"})
		return
	}
	if strings.ContainsAny(lang, "\n\r") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply_lang cannot contain a line break"})
		return
	}

	dic, err := config.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dic["reply_lang"] = lang
	if err := config.Write(dic); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filesystem.SetReplyLang(lang)

	c.JSON(http.StatusOK, gin.H{"ok": true, "reply_lang": lang})
}

func setOutputDir(c *gin.Context) {
	var body struct {
		OutputDir *string `json:"output_dir"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.OutputDir == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "output_dir is required"})
		return
	}

	raw := strings.TrimSpace(*body.OutputDir)
	if strings.ContainsAny(raw, "\n\r") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "output_dir cannot contain a line break"})
		return
	}
	resolved, err := filesystem.ResolveOutputDir(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dic, err := config.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dic["output_dir"] = raw
	if err := config.Write(dic); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filesystem.ConfigOutputDir = raw

	c.JSON(http.StatusOK, gin.H{"ok": true, "output_dir": raw, "resolved": resolved})
}

func setOfficialGuide(c *gin.Context) {
	var body struct {
		Enable *bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Enable == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enable is required"})
		return
	}

	dic, err := config.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dic["official_guide_disabled"] = !*body.Enable
	if err := config.Write(dic); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": *body.Enable})
}

func setSkillSource(c *gin.Context) {
	var body struct {
		Disabled *[]string `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Disabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "disabled is required"})
		return
	}

	for _, name := range *body.Disabled {
		if !slices.Contains(runtime.SkillSources, name) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown skill source: " + name})
			return
		}
	}
	disabled := []string{}
	for _, name := range runtime.SkillSources {
		if slices.Contains(*body.Disabled, name) {
			disabled = append(disabled, name)
		}
	}

	dic, err := config.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dic["skill_source_disabled"] = disabled
	if err := config.Write(dic); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if scanner := agents.Scanner(); scanner != nil {
		scanner.Scan()
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "disabled": disabled})
}

func setSecurity(c *gin.Context) {
	var body struct {
		CommandNetwork *bool `json:"command_network"`
		HookNetwork    *bool `json:"hook_network"`
		LANAccess      *bool `json:"lan_access"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.CommandNetwork == nil && body.HookNetwork == nil && body.LANAccess == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command_network, hook_network or lan_access is required"})
		return
	}

	dic, err := config.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if body.CommandNetwork != nil {
		dic["command_network_disabled"] = !*body.CommandNetwork
	}
	if body.HookNetwork != nil {
		dic["hook_network_disabled"] = !*body.HookNetwork
	}
	if body.LANAccess != nil {
		dic["lan_access_enabled"] = *body.LANAccess
	}
	if err := config.Write(dic); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
