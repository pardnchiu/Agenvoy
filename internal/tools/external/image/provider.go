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

const Off = ""

func Selected() string {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return Off
	}
	name := strings.TrimSpace(cfg.ImageGenerator)
	if name == "off" {
		return Off
	}
	return name
}

func Enabled() bool {
	return Selected() != Off
}

func Available(ctx context.Context) []string {
	filter := go_llm_router.ModelFilter{ImageOnly: true}
	found := make([][]string, len(Providers))

	var wg sync.WaitGroup
	for i, name := range Providers {
		wg.Go(func() {
			cfg, err := agentkeychain.Config(ctx, name+"@")
			if err != nil {
				return
			}
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

	list := []string{}
	for _, models := range found {
		list = append(list, models...)
	}
	return list
}

func Prune(ctx context.Context) {
	name := Selected()
	if name == Off {
		return
	}
	if !strings.Contains(name, "@") {
		if _, err := agentkeychain.Config(ctx, name+"@"); err == nil {
			return
		}
	} else if slices.Contains(Available(ctx), name) {
		return
	}

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return
	}
	cfg.ImageGenerator = Off
	if err := config.Save(cfg); err != nil {
		slog.Warn("image.Prune config.Save",
			slog.String("provider", name),
			slog.String("error", err.Error()))
	}
}

func agent(ctx context.Context) (go_llm_router.ImageAgent, string, error) {
	name := Selected()
	if name == Off {
		return nil, "", utilsmsg.New(utilsmsg.Unavailable, "image generation model", "not configured; ask the user to select one")
	}
	full := name
	if !strings.Contains(full, "@") {
		full += "@"
	}
	cfg, err := agentkeychain.Config(ctx, full)
	if err != nil {
		return nil, "", utilsmsg.Trace("internal/agents/keychain: Config", name, err)
	}
	built, err := router.New(router.Config{
		Name:      full,
		APIKey:    cfg.APIKey,
		Token:     cfg.Token,
		BaseURL:   cfg.BaseURL,
		AccountID: cfg.AccountID,
		GatewayID: cfg.GatewayID,
	})
	if err != nil {
		return nil, "", utilsmsg.Wrap(utilsmsg.Invalid, "router.New", full, err)
	}
	img, ok := built.(go_llm_router.ImageAgent)
	if !ok {
		return nil, "", utilsmsg.New(utilsmsg.Invalid, name, "configured but not an image generation model; ask the user to reconfigure it")
	}
	return img, full, nil
}
