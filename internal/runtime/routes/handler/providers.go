package handler

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/go-llm-router/core/claudeCode"
	oauthcodex "github.com/pardnchiu/go-llm-router/core/oauth/codex"
	oauthcopilot "github.com/pardnchiu/go-llm-router/core/oauth/copilot"
	oauthgrok "github.com/pardnchiu/go-llm-router/core/oauth/grok"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/compact"
	agentkeychain "github.com/pardnchiu/agenvoy/internal/agents/keychain"
	"github.com/pardnchiu/agenvoy/internal/agents/probe"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	audiotool "github.com/pardnchiu/agenvoy/internal/tools/external/audio"
	imagetool "github.com/pardnchiu/agenvoy/internal/tools/external/image"
)

type providerInfo struct {
	ID      string            `json:"id"`
	Label   string            `json:"label"`
	Methods map[string]string `json:"methods"`
}

type providerState struct {
	providerInfo
	LoggedIn bool     `json:"logged_in"`
	Console  []string `json:"console"`
	Hidden   bool     `json:"hidden"`
	Decision bool     `json:"decision"`
}

var providerConsole = map[string]map[string]string{
	"openai":       {"key": "https://platform.openai.com/api-keys", "billing": "https://platform.openai.com/settings/organization/billing"},
	"claude":       {"key": "https://platform.claude.com/settings/keys", "billing": "https://console.anthropic.com/settings/billing"},
	"gemini":       {"key": "https://aistudio.google.com/apikey", "billing": "https://aistudio.google.com/apikey"},
	"grok":         {"key": "https://console.x.ai/", "billing": "https://console.x.ai/"},
	"deepseek":     {"key": "https://platform.deepseek.com/api_keys", "billing": "https://platform.deepseek.com/top_up"},
	"mistral":      {"key": "https://console.mistral.ai/api-keys", "billing": "https://console.mistral.ai/billing"},
	"nvidia":       {"key": "https://build.nvidia.com/settings/api-keys", "billing": "https://build.nvidia.com/settings/api-keys"},
	"ollama-cloud": {"key": "https://ollama.com/settings/keys", "billing": "https://ollama.com/settings/keys"},
	"openrouter":   {"key": "https://openrouter.ai/settings/keys", "billing": "https://openrouter.ai/credits"},
	"typesafe":     {"key": "https://console.typesafe.ai/keys"},
	"cloudflare":   {"key": "https://dash.cloudflare.com/profile/api-tokens", "billing": "https://dash.cloudflare.com/profile/api-tokens"},
	"claude-code":  {"plan": "https://claude.com/pricing"},
	"codex":        {"plan": "https://chatgpt.com/pricing"},
	"grok-oauth":   {"plan": "https://grok.com/plans"},
	"copilot":      {"plan": "https://github.com/features/copilot/plans"},
}

var providerCatalog = []providerInfo{
	{"openai", "OpenAI", map[string]string{"api_key": "pay per token"}},
	{"codex", "OpenAI Codex", map[string]string{"oauth": "Codex subscription"}},
	{"claude", "Claude", map[string]string{"api_key": "pay per token"}},
	{"claude-code", "Claude Code", map[string]string{"oauth": "Claude subscription via claude -p"}},
	{"gemini", "Gemini", map[string]string{"api_key": "pay per token"}},
	{"grok", "Grok", map[string]string{"api_key": "pay per token"}},
	{"grok-oauth", "Grok (xAI)", map[string]string{"oauth": "xAI subscription"}},
	{"copilot", "GitHub Copilot", map[string]string{"oauth": "GitHub subscription"}},
	{"deepseek", "DeepSeek", map[string]string{"api_key": "pay per token"}},
	{"mistral", "Mistral", map[string]string{"api_key": "pay per token"}},
	{"nvidia", "NVIDIA NIM", map[string]string{"api_key": "pay per token"}},
	{"ollama-cloud", "Ollama Cloud", map[string]string{"api_key": "API key"}},
	{"openrouter", "OpenRouter", map[string]string{"api_key": "pay per token"}},
	{"typesafe", "TypeSafe", map[string]string{"api_key": "decision model"}},
	{"cloudflare", "Cloudflare", map[string]string{"api_key": "Workers AI · API token + account ID"}},
	{"compat", "Local/Custom", map[string]string{"custom": "Ollama, LM Studio, or custom URL"}},
}

func findProvider(id string) *providerInfo {
	for i := range providerCatalog {
		if providerCatalog[i].ID == id {
			return &providerCatalog[i]
		}
	}
	return nil
}

func providerLoggedIn(id string) bool {
	switch id {
	case "claude-code":
		return agenttypes.ClaudeCodeEnabled()
	case "codex":
		return oauthcodex.HasToken()
	case "copilot":
		return oauthcopilot.HasToken()
	case "grok-oauth":
		return oauthgrok.HasToken()
	}
	return false
}

func ListProviders() gin.HandlerFunc {
	return func(c *gin.Context) {
		list := make([]providerState, 0, len(providerCatalog))
		for _, provider := range providerCatalog {
			list = append(list, providerState{
				providerInfo: provider,
				LoggedIn:     providerLoggedIn(provider.ID),
				Console:      slices.Sorted(maps.Keys(providerConsole[provider.ID])),
				Hidden:       provider.ID == "claude-code" && !agenttypes.ClaudeCodeEnabled(),
				Decision:     provider.ID == "typesafe",
			})
		}
		c.JSON(http.StatusOK, gin.H{"providers": list})
	}
}

func ProviderConsole() gin.HandlerFunc {
	return func(c *gin.Context) {
		dic := providerConsole[c.Param("provider")]
		page := c.Query("page")
		if page == "" {
			page = "key"
			if _, ok := dic["plan"]; ok {
				page = "plan"
			}
		}
		link, ok := dic[page]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("no %s page for provider %q", page, c.Param("provider"))})
			return
		}
		c.Redirect(http.StatusFound, link)
	}
}

func AddProviderKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		prov := c.Param("provider")

		if prov == "compat" {
			var body struct {
				Name   string `json:"name"`
				URL    string `json:"url"`
				APIKey string `json:"api_key"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			name := strings.TrimSpace(body.Name)
			url := strings.TrimSpace(body.URL)
			if name == "" || url == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "name and url are required"})
				return
			}
			envKey := "COMPAT_API_KEY"
			if name != "" {
				envKey = "COMPAT_" + strings.ToUpper(name) + "_API_KEY"
			}
			if key := strings.TrimSpace(body.APIKey); key != "" {
				if err := keychain.Set(envKey, key); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				if err := config.SaveKey(envKey); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
			if err := config.UpsertCompat(name, url); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			DropQuotaCache(prov)
			imagetool.DropModelsCache(c.Request.Context(), prov)
			decision.DropModelsCache(c.Request.Context(), prov)
			audiotool.DropModelsCache(c.Request.Context(), prov)
			probe.DropModelsCache(c.Request.Context(), name)
			c.JSON(http.StatusOK, gin.H{"ok": true})
			return
		}

		p := findProvider(prov)
		if p == nil || p.Methods["api_key"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provider does not support an API key"})
			return
		}

		var body struct {
			APIKey    string `json:"api_key"`
			AccountID string `json:"account_id"`
			GatewayID string `json:"gateway_id"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		key := strings.TrimSpace(body.APIKey)
		if key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "api_key is required"})
			return
		}

		accountID := strings.TrimSpace(body.AccountID)
		if prov == "cloudflare" && accountID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "account_id is required for cloudflare"})
			return
		}

		envKey := strings.ToUpper(prov) + "_API_KEY"
		if err := keychain.Set(envKey, key); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := config.SaveKey(envKey); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		DropQuotaCache(prov)
		imagetool.DropModelsCache(c.Request.Context(), prov)
		decision.DropModelsCache(c.Request.Context(), prov)
		audiotool.DropModelsCache(c.Request.Context(), prov)
		probe.DropModelsCache(c.Request.Context(), prov)

		if prov == "cloudflare" {
			if err := keychain.Set("CLOUDFLARE_ACCOUNT_ID", accountID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if gatewayID := strings.TrimSpace(body.GatewayID); gatewayID != "" {
				if err := keychain.Set("CLOUDFLARE_GATEWAY_ID", gatewayID); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func ProviderOAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		prov := c.Param("provider")
		p := findProvider(prov)
		if p == nil || p.Methods["oauth"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provider does not support oauth"})
			return
		}

		h := c.Writer.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Flush()

		emit := func(v any) {
			raw, err := json.Marshal(v)
			if err != nil {
				return
			}
			fmt.Fprintf(c.Writer, "data: %s\n\n", raw)
			c.Writer.Flush()
		}

		ctx := c.Request.Context()
		var err error
		switch prov {
		case "claude-code":
			err = claudeCode.CheckBinary()
		case "copilot":
			_, err = oauthcopilot.LoginWithCallback(ctx, func(code *oauthcopilot.DeviceCode) {
				emit(gin.H{"url": code.VerificationURI, "user_code": code.UserCode})
			})
		case "codex":
			_, err = oauthcodex.LoginWithCallback(ctx, func(url string) {
				emit(gin.H{"url": url})
			})
		case "grok-oauth":
			_, err = oauthgrok.LoginWithCallback(ctx, func(url string) {
				emit(gin.H{"url": url})
			})
		}
		if err != nil {
			emit(gin.H{"done": true, "ok": false, "error": err.Error()})
			return
		}
		DropQuotaCache(prov)
		imagetool.DropModelsCache(c.Request.Context(), prov)
		decision.DropModelsCache(c.Request.Context(), prov)
		audiotool.DropModelsCache(c.Request.Context(), prov)
		probe.DropModelsCache(c.Request.Context(), prov)
		emit(gin.H{"done": true, "ok": true})
	}
}

func ClearProviderOAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		prov := c.Param("provider")
		p := findProvider(prov)
		if p == nil || p.Methods["oauth"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provider does not support oauth"})
			return
		}

		var err error
		switch prov {
		case "codex":
			err = oauthcodex.ClearToken()
		case "copilot":
			err = oauthcopilot.ClearToken()
		case "grok-oauth":
			err = oauthgrok.ClearToken()
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		DropQuotaCache(prov)
		imagetool.DropModelsCache(c.Request.Context(), prov)
		decision.DropModelsCache(c.Request.Context(), prov)
		audiotool.DropModelsCache(c.Request.Context(), prov)
		probe.DropModelsCache(c.Request.Context(), prov)
		imagetool.Prune(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func modelWindows(c *gin.Context, prefix string, ids []string) map[string]gin.H {
	dic := make(map[string]gin.H, len(ids))
	for _, id := range ids {
		in, out, ok := compact.Window(c.Request.Context(), prefix+"@"+id)
		if !ok {
			continue
		}
		dic[id] = gin.H{"in": in, "out": out}
	}
	return dic
}

func listModelsFor(c *gin.Context, credentialName string) {
	if !probe.Supports(probe.Provider(credentialName)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider does not support model listing"})
		return
	}
	ids, err := probe.Models(c.Request.Context(), credentialName)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": ids, "windows": modelWindows(c, credentialName, ids)})
}

func ListProviderModels() gin.HandlerFunc {
	return func(c *gin.Context) {
		prov := c.Param("provider")
		if _, ok := agentkeychain.CompatInstance(prov + "@"); ok {
			ids, err := probe.Models(c.Request.Context(), prov+"@")
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"models": ids, "windows": modelWindows(c, prov, ids)})
			return
		}
		listModelsFor(c, prov)
	}
}
