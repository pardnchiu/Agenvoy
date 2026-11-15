package utils

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	"github.com/pardnchiu/go-llm-router/core/claudeCode"
	"github.com/pardnchiu/go-llm-router/core/copilot"
	"github.com/pardnchiu/go-llm-router/core/deepseek"
	grokoauth "github.com/pardnchiu/go-llm-router/core/grokOauth"
	openrouter "github.com/pardnchiu/go-llm-router/core/openRouter"
	openaicodex "github.com/pardnchiu/go-llm-router/core/openaiCodex"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	agentkeychain "github.com/pardnchiu/agenvoy/internal/agents/keychain"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func FormatEventFooter(duration, outputElapsed time.Duration, model, reasoning string, usage *go_llm_router.Usage) string {
	var parts []string
	if duration > 0 {
		elapsed := duration.Round(100 * time.Millisecond).String()
		if outputElapsed > 0 && usage != nil && usage.Output > 0 {
			elapsed += fmt.Sprintf("(%.1ft/s)", float64(usage.Output)/outputElapsed.Seconds())
		}
		parts = append(parts, elapsed)
	}

	if model = strings.TrimSpace(model); model != "" {
		if reasoning != "" {
			model += "/" + reasoning
		}
		parts = append(parts, model)
	}

	if in := agenttypes.FormatInput(agenttypes.InputTotals(usage)); in != "" {
		parts = append(parts, fmt.Sprintf("↑ %s ↓ %s", in, go_pkg_utils.CompactNumber(usage.Output)))
	}
	return strings.Join(parts, "  ")
}

type QuotaSource struct {
	ID string
	Fn func(context.Context, go_llm_router.Config) (go_llm_router.UsageRemaining, error)
}

var QuotaSources = []QuotaSource{
	{"codex", openaicodex.Usage},
	{"grok-oauth", grokoauth.Usage},
	{"copilot", copilot.Usage},
	{"openrouter", openrouter.Usage},
	{"deepseek", deepseek.Usage},
	{"claude-code", claudeCode.Usage},
}

func FormatQuota(remaining go_llm_router.UsageRemaining) string {
	if remaining.Balance != nil {
		return fmt.Sprintf("$%.2f", *remaining.Balance)
	}
	list := []string{}
	for _, value := range []*float64{remaining.FiveHour, remaining.Week, remaining.Total} {
		if value != nil {
			list = append(list, fmt.Sprintf("%.0f%%", *value))
		}
	}
	return strings.Join(list, "/")
}

func ModelQuota(ctx context.Context, model string) string {
	prefix, _, ok := strings.Cut(strings.TrimSpace(model), "@")
	if !ok {
		return ""
	}
	i := slices.IndexFunc(QuotaSources, func(source QuotaSource) bool {
		return source.ID == prefix
	})
	if i < 0 {
		return ""
	}
	source := QuotaSources[i]

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cfg, err := agentkeychain.Config(ctx, source.ID)
	if err != nil {
		return ""
	}
	remaining, err := source.Fn(ctx, cfg)
	if err != nil {
		return ""
	}
	return FormatQuota(remaining)
}
