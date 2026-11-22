// * last modified: 26/11/18
package image

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	"github.com/pardnchiu/go-llm-router/core/gemini"
	"github.com/pardnchiu/go-llm-router/core/grok"
	grokoauth "github.com/pardnchiu/go-llm-router/core/grokOauth"
	openrouter "github.com/pardnchiu/go-llm-router/core/openRouter"
	"github.com/pardnchiu/go-llm-router/core/openai"
	"github.com/pardnchiu/go-llm-router/core/router"

	"github.com/pardnchiu/agenvoy/configs"
	agentkeychain "github.com/pardnchiu/agenvoy/internal/agents/keychain"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var Providers = []string{"openai", "codex", "grok", "grok-oauth", "gemini", "openrouter"}

func Model() string {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return ""
	}

	name := strings.TrimSpace(cfg.ImageGenerator)
	if name == "off" {
		return ""
	}
	return name
}

func Options(ctx context.Context) []string {
	filter := go_llm_router.ModelFilter{ImageOnly: true}
	found := make([][]string, len(Providers))

	var wg sync.WaitGroup
	for i, name := range Providers {
		wg.Go(func() {
			// NOTE: 檢查 keychain 是否存在
			cfg, err := agentkeychain.Config(ctx, name+"@")
			if err != nil {
				return
			}

			// NOTE: codex 沒有 list
			if name == "codex" {
				found[i] = []string{name}
				return
			}

			base := go_llm_router.Config{APIKey: cfg.APIKey, AccountID: cfg.AccountID}
			models, err := torii.CachedList(ctx, torii.ModelsCacheKey("image", name), configs.TTL_MODELS_CACHE_SEC, func() ([]string, error) {
				switch name {
				case "openai":
					return openai.Models(ctx, base, filter)
				case "grok":
					return grok.Models(ctx, base, filter)
				case "grok-oauth":
					return grokoauth.Models(ctx, base, filter)
				case "gemini":
					return gemini.Models(ctx, base, filter)
				case "openrouter":
					return openrouter.Models(ctx, base, filter)
				}
				return nil, nil
			})
			if err != nil {
				slog.Debug("image.Available",
					slog.String("provider", name),
					slog.String("error", err.Error()))
				return
			}

			for _, model := range models {
				found[i] = append(found[i], name+"@"+model)
			}
		})
	}
	wg.Wait()

	// NOTE: 統整並行搜集結果
	list := []string{}
	for _, models := range found {
		list = append(list, models...)
	}
	return list
}

func Prune(ctx context.Context) {
	name := Model()
	if name == "" {
		return
	}
	if !strings.Contains(name, "@") {
		if _, err := agentkeychain.Config(ctx, name+"@"); err == nil {
			return
		}
	} else if slices.Contains(Options(ctx), name) {
		return
	}

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return
	}
	cfg.ImageGenerator = ""
	if err := config.Save(cfg); err != nil {
		slog.Warn("image.Prune config.Save",
			slog.String("provider", name),
			slog.String("error", err.Error()))
	}
}

func new(ctx context.Context, name string) (go_llm_router.Agent, error) {
	cfg, err := agentkeychain.Config(ctx, name)
	if err != nil {
		return nil, utilsmsg.Trace("internal/agents/keychain: Config", name, err)
	}

	agent, err := router.New(router.Config{
		Name:      name,
		APIKey:    cfg.APIKey,
		Token:     cfg.Token,
		BaseURL:   cfg.BaseURL,
		AccountID: cfg.AccountID,
		GatewayID: cfg.GatewayID,
	})
	if err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Invalid, "router.New", name, err)
	}
	return agent, nil
}
