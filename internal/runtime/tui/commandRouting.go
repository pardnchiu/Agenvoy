package tui

import (
	"context"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/go-llm-router/core/claudeCode"

	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	audiotool "github.com/pardnchiu/agenvoy/internal/tools/external/audio"
	imagetool "github.com/pardnchiu/agenvoy/internal/tools/external/image"
)

const (
	summaryPrefix  = "summary:"
	imagePrefix    = "image:"
	decisionPrefix = "decision:"
	sttPrefix      = "stt:"
	ttsPrefix      = "tts:"
)

type RoutingTabLoaded struct {
	kind      string
	current   string
	available []string
	err       error
	target    *Popup
}

func summaryOptions() (options, values []string, cursor int) {
	cfg, err := config.Load()
	if err != nil || len(cfg.Models) == 0 {
		return nil, nil, 0
	}

	auto := "auto"
	if cfg.SummaryModel == "" {
		auto += "  " + systemStyle.Render("[current]")
	}
	options = append(options, auto)
	values = append(values, summaryPrefix)

	skipClaudeCode := !agenttypes.ClaudeCodeEnabled()
	for _, m := range cfg.Models {
		if skipClaudeCode && claudeCode.Is(m.Name) {
			continue
		}
		label := m.Name
		if cfg.SummaryModel != "" && m.Name == cfg.SummaryModel {
			label += "  " + systemStyle.Render("[current]")
			cursor = len(options)
		}
		options = append(options, label)
		values = append(values, summaryPrefix+m.Name)
	}
	return options, values, cursor
}

func routingTabLoad(p *Popup, kind string) tea.Cmd {
	p.options, p.values, p.cursor = nil, nil, 0
	p.styledLines = []string{hintStyle.Render("  loading...")}

	target := p
	return func() tea.Msg {
		ctx := context.Background()
		cfg, err := config.Load()
		if err != nil {
			return RoutingTabLoaded{kind: kind, err: err, target: target}
		}
		switch kind {
		case "image":
			imagetool.Prune(ctx)
			return RoutingTabLoaded{kind: kind, current: cfg.ImageGenerator, available: imagetool.Available(ctx), target: target}
		case "decision":
			return RoutingTabLoaded{kind: kind, current: cfg.DecisionModel, available: decision.Available(ctx), target: target}
		case "stt":
			return RoutingTabLoaded{kind: kind, current: cfg.STTModel, available: audiotool.STTOptions(ctx), target: target}
		}
		return RoutingTabLoaded{kind: kind, current: cfg.TTSModel, available: audiotool.TTSOptions(ctx), target: target}
	}
}

func (t TUI) runRoutingTabLoaded(msg RoutingTabLoaded) (TUI, tea.Cmd) {
	p := t.popup
	if p != msg.target {
		return t, nil
	}
	if msg.err != nil {
		p.styledLines = []string{errorStyle.Render("  config.Load: " + msg.err.Error())}
		return t, nil
	}
	if len(msg.available) == 0 {
		p.options, p.values, p.cursor = nil, nil, 0
		p.styledLines = []string{hintStyle.Render("  no provider with credentials  add one from the model tab")}
		return t, nil
	}

	prefix := map[string]string{"image": imagePrefix, "decision": decisionPrefix, "stt": sttPrefix, "tts": ttsPrefix}[msg.kind]
	disable := "disable"
	if msg.current == "" {
		disable += "  " + systemStyle.Render("[current]")
	}
	options := []string{disable}
	values := []string{prefix}
	cursor := 0
	for _, name := range msg.available {
		label := name
		if name == msg.current {
			label += "  " + systemStyle.Render("[current]")
			cursor = len(options)
		}
		options = append(options, label)
		values = append(values, prefix+name)
	}

	p.styledLines = nil
	p.options, p.values, p.cursor = options, values, cursor

	if slices.ContainsFunc(msg.available, func(name string) bool { return strings.HasPrefix(name, "openrouter@") }) {
		input := newPopupInput("", false)
		input.Placeholder = "Search models..."
		input.SetPromptFunc(2, func(int) string {
			return hintStyle.Render("/ ")
		})
		p.input = input
		p.allOptions, p.allValues = options, values
		p.searchable = true
	}
	return t, nil
}
