package audio

import (
	"context"
	"strings"
	"sync"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	"github.com/pardnchiu/go-llm-router/core/gemini"
	openrouter "github.com/pardnchiu/go-llm-router/core/openRouter"
	"github.com/pardnchiu/go-llm-router/core/openai"
	"github.com/pardnchiu/go-llm-router/core/router"

	"github.com/pardnchiu/agenvoy/configs"
	agentkeychain "github.com/pardnchiu/agenvoy/internal/agents/keychain"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var Providers = []string{"openai", "gemini", "openrouter"}

const (
	STT = iota
	TTS
)

func Model(target int) string {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return ""
	}

	var name string
	switch target {
	case STT:
		name = strings.TrimSpace(cfg.STTModel)
	case TTS:
		name = strings.TrimSpace(cfg.TTSModel)
	}
	if name == "off" {
		return ""
	}
	return name
}

func Options(ctx context.Context, target int) []string {
	var filter go_llm_router.ModelFilter
	var kind string
	switch target {
	case STT:
		filter = go_llm_router.ModelFilter{STTOnly: true}
		kind = "stt"
	case TTS:
		filter = go_llm_router.ModelFilter{TTSOnly: true}
		kind = "tts"
	default:
		return []string{}
	}
	found := make([][]string, len(Providers))

	var wg sync.WaitGroup
	for i, name := range Providers {
		wg.Go(func() {
			// NOTE: 檢查 keychain 是否存在
			cfg, err := agentkeychain.Config(ctx, name+"@")
			if err != nil {
				return
			}

			// NOTE: 快取模型結果 15m 減少 delay
			models, err := torii.CachedList(ctx, torii.ModelsCacheKey(kind, name), configs.TTL_MODELS_CACHE_SEC, func() ([]string, error) {
				switch name {
				case "openai":
					return openai.Models(ctx, go_llm_router.Config{APIKey: cfg.APIKey}, filter)
				case "gemini":
					return gemini.Models(ctx, go_llm_router.Config{APIKey: cfg.APIKey}, filter)
				case "openrouter":
					return openrouter.Models(ctx, go_llm_router.Config{APIKey: cfg.APIKey}, filter)
				}
				return nil, nil
			})
			if err != nil {
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

func new(ctx context.Context, name string) (any, error) {
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
