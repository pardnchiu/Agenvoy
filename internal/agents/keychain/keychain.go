package keychain

import (
	"context"
	"slices"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	"github.com/pardnchiu/go-llm-router/core/claudeCode"
	oauthcodex "github.com/pardnchiu/go-llm-router/core/oauth/codex"
	oauthcopilot "github.com/pardnchiu/go-llm-router/core/oauth/copilot"
	oauthgrok "github.com/pardnchiu/go-llm-router/core/oauth/grok"
	go_pkg_keychain "github.com/pardnchiu/go-pkg/filesystem/keychain"

	sessionconfig "github.com/pardnchiu/agenvoy/internal/session/config"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Config(ctx context.Context, name string) (go_llm_router.Config, error) {
	providerFull, _, _ := strings.Cut(name, "@")
	prov, _, _ := strings.Cut(providerFull, "[")

	switch prov {
	case "claude-code":
		if err := claudeCode.CheckBinary(); err != nil {
			return go_llm_router.Config{}, err
		}
		return go_llm_router.Config{}, nil

	case "claude":
		apiKey := go_pkg_keychain.Get("CLAUDE_API_KEY")
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain CLAUDE_API_KEY", "required")
		}
		return go_llm_router.Config{APIKey: apiKey}, nil

	case "openai":
		apiKey := go_pkg_keychain.Get("OPENAI_API_KEY")
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain OPENAI_API_KEY", "required")
		}
		return go_llm_router.Config{APIKey: apiKey}, nil

	case "gemini":
		apiKey := go_pkg_keychain.Get("GEMINI_API_KEY")
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain GEMINI_API_KEY", "required")
		}
		return go_llm_router.Config{APIKey: apiKey}, nil

	case "grok":
		apiKey := go_pkg_keychain.Get("GROK_API_KEY")
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain GROK_API_KEY", "required")
		}
		return go_llm_router.Config{APIKey: apiKey}, nil

	case "deepseek":
		apiKey := go_pkg_keychain.Get("DEEPSEEK_API_KEY")
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain DEEPSEEK_API_KEY", "required")
		}
		return go_llm_router.Config{APIKey: apiKey}, nil

	case "mistral":
		apiKey := go_pkg_keychain.Get("MISTRAL_API_KEY")
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain MISTRAL_API_KEY", "required")
		}
		return go_llm_router.Config{APIKey: apiKey}, nil

	case "nvidia":
		apiKey := go_pkg_keychain.Get("NVIDIA_API_KEY")
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain NVIDIA_API_KEY", "required")
		}
		return go_llm_router.Config{APIKey: apiKey}, nil

	case "ollama-cloud":
		apiKey := go_pkg_keychain.Get("OLLAMA-CLOUD_API_KEY")
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain OLLAMA-CLOUD_API_KEY", "required")
		}
		return go_llm_router.Config{APIKey: apiKey}, nil

	case "openrouter":
		apiKey := go_pkg_keychain.Get("OPENROUTER_API_KEY")
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain OPENROUTER_API_KEY", "required")
		}
		return go_llm_router.Config{APIKey: apiKey}, nil

	case "typesafe":
		apiKey := go_pkg_keychain.Get(sessionconfig.TypesafeKey)
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain "+sessionconfig.TypesafeKey, "required")
		}
		return go_llm_router.Config{APIKey: apiKey}, nil

	case "cloudflare":
		apiKey := go_pkg_keychain.Get("CLOUDFLARE_API_KEY")
		if apiKey == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain CLOUDFLARE_API_KEY", "required")
		}
		accountID := go_pkg_keychain.Get("CLOUDFLARE_ACCOUNT_ID")
		if accountID == "" {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "keychain CLOUDFLARE_ACCOUNT_ID", "required")
		}
		return go_llm_router.Config{
			APIKey:    apiKey,
			AccountID: accountID,
			GatewayID: go_pkg_keychain.Get("CLOUDFLARE_GATEWAY_ID"),
		}, nil

	case "copilot":
		token, err := oauthcopilot.Load()
		if err != nil {
			return go_llm_router.Config{}, utilsmsg.Wrap(utilsmsg.Internal, "oauthcopilot.Load", "", err)
		}
		if token == nil {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "copilot token", "missing; run `agen model add` to authenticate")
		}
		refresh, err := oauthcopilot.EnsureFreshSession(ctx, token, nil)
		if err != nil {
			return go_llm_router.Config{}, utilsmsg.Wrap(utilsmsg.Upstream, "oauthcopilot.EnsureFreshSession", "", err)
		}
		return go_llm_router.Config{APIKey: refresh.Token, Token: token}, nil

	case "codex":
		token, err := oauthcodex.Load()
		if err != nil {
			return go_llm_router.Config{}, utilsmsg.Wrap(utilsmsg.Internal, "oauthcodex.Load", "", err)
		}
		if token == nil {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "codex token", "missing; run `agen model add` to authenticate")
		}
		token, err = oauthcodex.EnsureFresh(ctx, token)
		if err != nil {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Upstream, "codex token", "expired and refresh failed: %w; run `agen model add` to re-authenticate", err)
		}
		return go_llm_router.Config{APIKey: token.AccessToken, AccountID: token.AccountID, Token: token}, nil

	case "grok-oauth":
		token, err := oauthgrok.Load()
		if err != nil {
			return go_llm_router.Config{}, utilsmsg.Wrap(utilsmsg.Internal, "oauthgrok.Load", "", err)
		}
		if token == nil {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Denied, "grok-oauth token", "missing; run `agen model add` to authenticate")
		}
		token, err = oauthgrok.EnsureFresh(ctx, token)
		if err != nil {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Upstream, "grok-oauth token", "expired and refresh failed: %w; run `agen model add` to re-authenticate", err)
		}
		return go_llm_router.Config{APIKey: token.AccessToken, Token: token}, nil

	default:
		instance, ok := CompatInstance(name)
		if !ok {
			return go_llm_router.Config{}, utilsmsg.New(utilsmsg.Invalid, "provider", "unknown %q in %q", prov, name)
		}
		apiKeyEnvKey := "COMPAT_API_KEY"
		if instance != "" {
			apiKeyEnvKey = "COMPAT_" + instance + "_API_KEY"
		}
		return go_llm_router.Config{
			APIKey:  go_pkg_keychain.Get(apiKeyEnvKey),
			BaseURL: sessionconfig.GetCompatURL(instance),
		}, nil
	}
}

func CompatInstance(name string) (string, bool) {
	providerFull, _, found := strings.Cut(name, "@")
	if !found {
		return "", false
	}
	prov, rest, bracket := strings.Cut(providerFull, "[")
	if prov == "compat" {
		if !bracket {
			return "", true
		}
		instance, _, _ := strings.Cut(rest, "]")
		return strings.ToUpper(instance), true
	}
	if prov == "" || slices.Contains(go_llm_router.Providers(), prov) {
		return "", false
	}
	return strings.ToUpper(prov), true
}
