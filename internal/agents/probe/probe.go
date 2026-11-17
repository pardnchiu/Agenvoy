package probe

import (
	"context"
	"net/http"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	"github.com/pardnchiu/go-llm-router/core/claude"
	"github.com/pardnchiu/go-llm-router/core/claudeCode"
	"github.com/pardnchiu/go-llm-router/core/cloudflare"
	"github.com/pardnchiu/go-llm-router/core/copilot"
	"github.com/pardnchiu/go-llm-router/core/deepseek"
	"github.com/pardnchiu/go-llm-router/core/gemini"
	"github.com/pardnchiu/go-llm-router/core/grok"
	grokoauth "github.com/pardnchiu/go-llm-router/core/grokOauth"
	"github.com/pardnchiu/go-llm-router/core/mistral"
	"github.com/pardnchiu/go-llm-router/core/nvidia"
	ollamacloud "github.com/pardnchiu/go-llm-router/core/ollamaCloud"
	openrouter "github.com/pardnchiu/go-llm-router/core/openRouter"
	"github.com/pardnchiu/go-llm-router/core/openai"
	openaicodex "github.com/pardnchiu/go-llm-router/core/openaiCodex"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"

	"github.com/pardnchiu/agenvoy/configs"
	agentkeychain "github.com/pardnchiu/agenvoy/internal/agents/keychain"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type listFn func(context.Context, go_llm_router.Config) ([]string, error)

func lookup(prov string) listFn {
	filter := go_llm_router.ModelFilter{TextOnly: true}
	switch prov {
	case "claude-code":
		return claudeCode.Models
	case "openai":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return openai.Models(ctx, cfg, filter)
		}
	case "codex":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return openaicodex.Models(ctx, cfg, filter)
		}
	case "claude":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return claude.Models(ctx, cfg, filter)
		}
	case "gemini":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return gemini.Models(ctx, cfg, filter)
		}
	case "grok":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return grok.Models(ctx, cfg, filter)
		}
	case "grok-oauth":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return grokoauth.Models(ctx, cfg, filter)
		}
	case "copilot":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return copilot.Models(ctx, cfg, filter)
		}
	case "deepseek":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return deepseek.Models(ctx, cfg, filter)
		}
	case "mistral":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return mistral.Models(ctx, cfg, filter)
		}
	case "nvidia":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return nvidia.Models(ctx, cfg, filter)
		}
	case "openrouter":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return openrouter.Models(ctx, cfg, filter)
		}
	case "ollama-cloud":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return ollamacloud.Models(ctx, cfg, filter)
		}
	case "cloudflare":
		return func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return cloudflare.Models(ctx, cfg, filter)
		}
	default:
		return nil
	}
}

func Supports(prov string) bool {
	return lookup(prov) != nil
}

func Models(ctx context.Context, name string) ([]string, error) {
	prov := Provider(name)
	fn := lookup(prov)
	if fn == nil {
		if _, ok := agentkeychain.CompatInstance(name); !ok {
			return nil, utilsmsg.New(utilsmsg.Invalid, "provider", "%q does not support model listing", prov)
		}
		fn = func(ctx context.Context, cfg go_llm_router.Config) ([]string, error) {
			return CompatModels(ctx, cfg.BaseURL, cfg.APIKey)
		}
	}
	return torii.CachedList(ctx, torii.ModelsCacheKey("list", strings.TrimSuffix(name, "@")), configs.TTL_MODELS_CACHE_SEC, func() ([]string, error) {
		cfg, err := agentkeychain.Config(ctx, name)
		if err != nil {
			return nil, err
		}
		return fn(ctx, cfg)
	})
}

func CompatModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return nil, utilsmsg.New(utilsmsg.Invalid, "compat base url", "not configured")
	}

	var headers map[string]string
	if apiKey != "" {
		headers = map[string]string{"Authorization": "Bearer " + apiKey}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	data, status, err := go_pkg_http.GET[compatModelList](ctx, client, baseURL+"/models", headers)
	if err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Upstream, "go_pkg_http.GET", baseURL+"/models", err)
	}
	if status != http.StatusOK {
		return nil, utilsmsg.New(utilsmsg.Upstream, baseURL+"/models", "http %d", status)
	}

	ids := make([]string, 0, len(data.Data))
	for _, one := range data.Data {
		if id := strings.TrimSpace(one.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

type compatModelList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func Provider(name string) string {
	providerFull, _, _ := strings.Cut(name, "@")
	prov, _, _ := strings.Cut(providerFull, "[")
	return prov
}

func Alive(ctx context.Context, name string, timeout time.Duration) bool {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	models, err := Models(probeCtx, name)
	return err == nil && len(models) > 0
}
