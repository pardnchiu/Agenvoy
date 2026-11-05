package tui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

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
	oauthcodex "github.com/pardnchiu/go-llm-router/core/oauth/codex"
	oauthcopilot "github.com/pardnchiu/go-llm-router/core/oauth/copilot"
	oauthgrok "github.com/pardnchiu/go-llm-router/core/oauth/grok"
	ollamacloud "github.com/pardnchiu/go-llm-router/core/ollamaCloud"
	openrouter "github.com/pardnchiu/go-llm-router/core/openRouter"
	"github.com/pardnchiu/go-llm-router/core/openai"
	openaicodex "github.com/pardnchiu/go-llm-router/core/openaiCodex"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/compact"
	agentkeychain "github.com/pardnchiu/agenvoy/internal/agents/keychain"
	"github.com/pardnchiu/agenvoy/internal/agents/probe"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime/daemon"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type modelAddItem struct {
	provider       string
	compatProvider string
	compatURL      string
	loading        *Popup
}

type ModelAddProviderPick struct{ provider string }
type ModelAddAPIKeyReplace struct{ replace string }
type ModelAddAPIKeySubmit struct{ key string }
type ModelAddVectorNotice struct{}
type ModelAddCompatNameSubmit struct{ name string }
type ModelAddCompatURLSubmit struct{ url string }
type ModelAddCompatKeySubmit struct{ key string }
type ModelAddModelMultiPick struct{ chosen string }
type ModelAddDone struct{ err error }
type ModelAddAccountIDReplace struct{ replace string }
type ModelAddAccountIDSubmit struct{ id string }
type ModelAddGatewayIDPick struct{ chosen string }
type ModelAddGatewayIDSubmit struct{ id string }
type CompatModelsResult struct {
	ids     []string
	windows map[string]string
}

type RemoteModelsResult struct {
	ids     []string
	windows map[string]string
}

type OAuthInfo struct {
	url      string
	userCode string
}
type OAuthSuccess struct{ provider string }
type OAuthFailed struct{ err error }
type OAuthReLoginPick struct{ replace string }

type modelAddMethod struct {
	key    string
	detail string
	value  string
}

var modelAddProviders = []struct {
	tab     string
	methods []modelAddMethod
}{
	{"OAuth", []modelAddMethod{
		{"OpenAI Codex", "Codex subscription", "codex"},
		{"Claude Code", "Claude subscription via claude -p", "claude-code"},
		{"Grok (xAI)", "xAI subscription", "grok-oauth"},
		{"GitHub Copilot", "GitHub subscription", "copilot"},
	}},
	{"API Key", []modelAddMethod{
		{"OpenAI", "pay per token", "openai"},
		{"Claude", "pay per token", "claude"},
		{"Grok", "pay per token", "grok"},
		{"Gemini", "pay per token", "gemini"},
		{"DeepSeek", "pay per token", "deepseek"},
		{"Mistral", "pay per token", "mistral"},
		{"OpenRouter", "pay per token", "openrouter"},
		{"Ollama Cloud", "API key", "ollama-cloud"},
		{"NVIDIA NIM", "pay per token", "nvidia"},
		{"TypeSafe", "decision model", "typesafe"},
	}},
	{"Custom", []modelAddMethod{
		{"Cloudflare", "Workers AI  API token + account ID", "cloudflare"},
		{"Local/Custom", "Ollama, LM Studio, or custom URL", "compat"},
	}},
}

const localCompatPrefix = "local:"

func (t TUI) commandModelAdd() (TUI, tea.Cmd, bool) {
	t.modelAdd = &modelAddItem{}

	found := make([]bool, len(config.LocalCompats))
	var consoles []string
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	var wg sync.WaitGroup
	for i, one := range config.LocalCompats {
		wg.Go(func() {
			found[i] = len(fetchModelIDs(ctx, one.URL, one.Provider)) > 0
		})
	}
	wg.Go(func() {
		consoles = fetchProviderConsoles(ctx)
	})
	wg.Wait()
	cancel()

	var locals []modelAddMethod
	for i, one := range config.LocalCompats {
		if found[i] {
			locals = append(locals, modelAddMethod{one.Label, one.URL, localCompatPrefix + one.Provider})
		}
	}

	tabs := make([]string, 0, len(modelAddProviders))
	for _, p := range modelAddProviders {
		tabs = append(tabs, p.tab)
	}

	popup := &Popup{
		kind:  popupSingleSelect,
		title: "/model add",
		tabs:  tabs,
		onConfirm: func(chosen string) any {
			return ModelAddProviderPick{provider: chosen}
		},
		openLabel: "console",
		onOpen: func(chosen string) string {
			if !slices.Contains(consoles, chosen) {
				return ""
			}
			return daemon.BaseURL() + "/v1/provider/" + url.PathEscape(chosen) + "/console"
		},
	}
	popup.onTab = func(p *Popup) tea.Cmd {
		methods := modelAddProviders[p.tabIdx].methods
		if modelAddProviders[p.tabIdx].tab == "Custom" {
			methods = append(slices.Clone(methods), locals...)
		}
		keys := make([]string, 0, len(methods))
		details := make([]string, 0, len(methods))
		values := make([]string, 0, len(methods))
		for _, m := range methods {
			if m.value == "claude-code" && !agenttypes.ClaudeCodeEnabled() {
				continue
			}
			keys = append(keys, m.key)
			details = append(details, m.detail)
			values = append(values, m.value)
		}
		p.options = optionColumn(keys, details)
		p.values = values
		p.cursor = 0
		return nil
	}
	popup.onTab(popup)
	t.popup = popup
	return t, nil, true
}

func fetchProviderConsoles(ctx context.Context) []string {
	resp, err := daemon.Get[struct {
		Providers []struct {
			ID      string   `json:"id"`
			Console []string `json:"console"`
		} `json:"providers"`
	}](ctx, "/v1/providers", nil)
	if err != nil {
		slog.Debug("fetchProviderConsoles", slog.String("error", err.Error()))
		return nil
	}
	var list []string
	for _, one := range resp.Providers {
		if len(one.Console) > 0 {
			list = append(list, one.ID)
		}
	}
	return list
}

func (t TUI) runModelAddProviderPick(name string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		t.modelAdd = &modelAddItem{}
	}
	if instance, ok := strings.CutPrefix(name, localCompatPrefix); ok {
		return t.runModelAddLocal(instance)
	}
	t.modelAdd.provider = name
	switch name {
	case "cloudflare":
		return t.openModelAddAccountID()
	case "copilot", "codex", "grok-oauth":
		return t.modelAddViaOAuth()
	case "compat":
		return t.openModelAddCompatURL()
	case "claude-code":
		if err := claudeCode.CheckBinary(); err != nil {
			t.modelAdd = nil
			return t, notice(msgError(err.Error()) + "\n")
		}
		return t.openModelAddModelPick()
	default:
		return t.openModelAddAPIKey()
	}
}

func (t TUI) modelAddViaOAuth() (TUI, tea.Cmd) {
	prov := t.modelAdd.provider
	hasToken := false
	switch prov {
	case "copilot":
		hasToken = oauthcopilot.HasToken()
	case "codex":
		hasToken = oauthcodex.HasToken()
	case "grok-oauth":
		hasToken = oauthgrok.HasToken()
	}
	if hasToken {
		label := strings.ToUpper(prov[:1]) + prov[1:]
		t.popup = &Popup{
			kind:    popupSingleSelect,
			title:   fmt.Sprintf("%s token exists  re-login?", label),
			options: []string{"No   keep existing", "Yes  re-authenticate"},
			values:  []string{"no", "yes"},
			onConfirm: func(chosen string) any {
				return OAuthReLoginPick{replace: chosen}
			},
		}
		return t, nil
	}
	return t.startOAuthPopup()
}

func (t TUI) startOAuthPopup() (TUI, tea.Cmd) {
	prov := t.modelAdd.provider
	ctx, cancel := context.WithTimeout(t.ctx, 15*time.Minute)

	t.popup = &Popup{
		kind:     popupOAuth,
		title:    fmt.Sprintf("%s OAuth  waiting for device code...", strings.ToUpper(prov[:1])+prov[1:]),
		subtitle: "browser will open automatically once the code is ready",
		oauth: &oauthState{
			provider: prov,
			cancel:   cancel,
		},
	}

	go runOAuthFlow(ctx, prov)
	return t, nil
}

func (t TUI) runOAuthReLoginPick(replace string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}
	if replace == "yes" {
		return t.startOAuthPopup()
	}
	return t.openModelAddModelPick()
}

func runOAuthFlow(ctx context.Context, prov string) {
	var err error
	switch prov {
	case "copilot":
		_, err = oauthcopilot.LoginWithCallback(ctx, func(code *oauthcopilot.DeviceCode) {
			send(OAuthInfo{
				url:      code.VerificationURI,
				userCode: code.UserCode,
			})
		})
	case "codex":
		_, err = oauthcodex.LoginWithCallback(ctx, func(url string) {
			send(OAuthInfo{url: url})
		})
	case "grok-oauth":
		_, err = oauthgrok.LoginWithCallback(ctx, func(url string) {
			send(OAuthInfo{url: url})
		})
	default:
		err = utilsmsg.New(utilsmsg.Invalid, "oauth provider", "unsupported %s", prov)
	}
	if err != nil {
		send(OAuthFailed{err: err})
		return
	}
	send(OAuthSuccess{provider: prov})
}

func (t TUI) runOAuthInfo(msg OAuthInfo) (TUI, tea.Cmd) {
	if t.popup == nil || t.popup.kind != popupOAuth || t.popup.oauth == nil {
		return t, nil
	}
	t.popup.oauth.url = msg.url
	t.popup.oauth.userCode = msg.userCode
	title := "OAuth  open browser to authorize"
	if t.popup.oauth.provider != "" {
		label := strings.ToUpper(t.popup.oauth.provider[:1]) + t.popup.oauth.provider[1:]
		title = fmt.Sprintf("%s OAuth  open browser to authorize", label)
	}
	t.popup.title = title
	t.popup.subtitle = ""
	if msg.url != "" && msg.userCode == "" {
		openBrowser(msg.url)
	}
	return t, nil
}

func (t TUI) runOAuthSuccess(msg OAuthSuccess) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		t.modelAdd = &modelAddItem{provider: msg.provider}
	}
	t.popup = nil
	return t.openModelAddModelPick()
}

func (t TUI) runOAuthFailed(msg OAuthFailed) (TUI, tea.Cmd) {
	t.modelAdd = nil
	t.popup = nil
	switch {
	case errors.Is(msg.err, context.Canceled):
		return t, nil
	case errors.Is(msg.err, context.DeadlineExceeded):
		return t, notice(msgWarn("oauth timed out  device code expired") + "\n")
	}
	return t, notice(msgError(fmt.Sprintf("oauth: %v", msg.err)) + "\n")
}

func (t TUI) openModelAddAPIKey() (TUI, tea.Cmd) {
	envKey := strings.ToUpper(t.modelAdd.provider) + "_API_KEY"
	label := strings.ToUpper(t.modelAdd.provider[:1]) + t.modelAdd.provider[1:]
	if keychain.Get(envKey) != "" {
		t.popup = &Popup{
			kind:    popupSingleSelect,
			title:   fmt.Sprintf("%s API key exists  replace?", label),
			options: []string{"No   keep existing", "Yes  overwrite"},
			values:  []string{"no", "yes"},
			onConfirm: func(chosen string) any {
				return ModelAddAPIKeyReplace{replace: chosen}
			},
		}
		return t, nil
	}
	t.popup = &Popup{
		kind:  popupSecret,
		title: fmt.Sprintf("%s API key", label),
		input: newPopupInput("", false),
		onConfirm: func(value string) any {
			return ModelAddAPIKeySubmit{key: value}
		},
	}
	return t, nil
}

func (t TUI) runModelAddAPIKeyReplace(replace string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}
	if replace == "yes" {
		label := strings.ToUpper(t.modelAdd.provider[:1]) + t.modelAdd.provider[1:]
		t.popup = &Popup{
			kind:  popupSecret,
			title: fmt.Sprintf("%s API key (new)", label),
			input: newPopupInput("", false),
			onConfirm: func(value string) any {
				return ModelAddAPIKeySubmit{key: value}
			},
		}
		return t, nil
	}
	if t.modelAdd.provider == "typesafe" {
		return t.finishModelAddDecisionKey()
	}
	return t.openModelAddModelPick()
}

func (t TUI) runModelAddAPIKeySubmit(key string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		t.modelAdd = nil
		return t, notice(msgError("api key required") + "\n")
	}
	envKey := strings.ToUpper(t.modelAdd.provider) + "_API_KEY"
	if err := keychain.Set(envKey, key); err != nil {
		t.modelAdd = nil
		return t, notice(msgError(fmt.Sprintf("keychain.Set: %v", err)) + "\n")
	}
	if err := config.SaveKey(envKey); err != nil {
		t.modelAdd = nil
		return t, notice(msgError(fmt.Sprintf("session.SaveKey: %v", err)) + "\n")
	}
	if t.modelAdd.provider == "cloudflare" {
		return t.openModelAddGatewayID()
	}
	if t.modelAdd.provider == "openai" {
		return t.openVectorRestartNotice()
	}
	if t.modelAdd.provider == "typesafe" {
		return t.finishModelAddDecisionKey()
	}
	return t.openModelAddModelPick()
}

func (t TUI) finishModelAddDecisionKey() (TUI, tea.Cmd) {
	decision.DropModelsCache(context.Background(), t.modelAdd.provider)
	t.modelAdd = nil
	return t, notice(msgLog("TypeSafe API key saved; pick its model in /model decision") + "\n")
}

func (t TUI) openVectorRestartNotice() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    "OpenAI key saved  vector storage needs a daemon restart",
		subtitle: "run  agen stop && agen  to enable it  entries written before the restart keep no vector",
		options:  []string{"OK"},
		values:   []string{"ok"},
		onConfirm: func(string) any {
			return ModelAddVectorNotice{}
		},
	}
	return t, nil
}

func (t TUI) openModelAddAccountID() (TUI, tea.Cmd) {
	if keychain.Get("CLOUDFLARE_ACCOUNT_ID") != "" {
		t.popup = &Popup{
			kind:    popupSingleSelect,
			title:   "Cloudflare account ID exists  replace?",
			options: []string{"No   keep existing", "Yes  overwrite"},
			values:  []string{"no", "yes"},
			onConfirm: func(chosen string) any {
				return ModelAddAccountIDReplace{replace: chosen}
			},
		}
		return t, nil
	}
	t.popup = &Popup{
		kind:  popupText,
		title: "Cloudflare account ID",
		input: newPopupInput("", false),
		onConfirm: func(value string) any {
			return ModelAddAccountIDSubmit{id: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) runModelAddAccountIDReplace(replace string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}
	if replace == "yes" {
		t.popup = &Popup{
			kind:  popupText,
			title: "Cloudflare account ID (new)",
			input: newPopupInput("", false),
			onConfirm: func(value string) any {
				return ModelAddAccountIDSubmit{id: strings.TrimSpace(value)}
			},
		}
		return t, nil
	}
	return t.openModelAddAPIKey()
}

func (t TUI) runModelAddAccountIDSubmit(id string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}
	if id == "" {
		t.modelAdd = nil
		return t, notice(msgError("account ID required") + "\n")
	}
	if err := keychain.Set("CLOUDFLARE_ACCOUNT_ID", id); err != nil {
		t.modelAdd = nil
		return t, notice(msgError(fmt.Sprintf("keychain.Set: %v", err)) + "\n")
	}
	return t.openModelAddAPIKey()
}

func (t TUI) openModelAddGatewayID() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Cloudflare AI Gateway ID",
		options: []string{"default", "Custom   enter gateway ID"},
		values:  []string{"default", "custom"},
		onConfirm: func(chosen string) any {
			return ModelAddGatewayIDPick{chosen: chosen}
		},
	}
	return t, nil
}

func (t TUI) runModelAddGatewayIDPick(chosen string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}
	if chosen == "custom" {
		t.popup = &Popup{
			kind:  popupText,
			title: "Cloudflare AI Gateway ID",
			input: newPopupInput("", false),
			onConfirm: func(value string) any {
				return ModelAddGatewayIDSubmit{id: strings.TrimSpace(value)}
			},
		}
		return t, nil
	}
	if err := keychain.Set("CLOUDFLARE_GATEWAY_ID", "default"); err != nil {
		t.modelAdd = nil
		return t, notice(msgError(fmt.Sprintf("keychain.Set: %v", err)) + "\n")
	}
	return t.openModelAddModelPick()
}

func (t TUI) runModelAddGatewayIDSubmit(id string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}
	if id == "" {
		t.modelAdd = nil
		return t, notice(msgError("gateway ID required") + "\n")
	}
	if err := keychain.Set("CLOUDFLARE_GATEWAY_ID", id); err != nil {
		t.modelAdd = nil
		return t, notice(msgError(fmt.Sprintf("keychain.Set: %v", err)) + "\n")
	}
	return t.openModelAddModelPick()
}

func (t TUI) openModelAddCompatName() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:  popupText,
		title: "Compat provider name (ex. ollama)",
		input: newPopupInput("", false),
		onConfirm: func(value string) any {
			return ModelAddCompatNameSubmit{name: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) runModelAddCompatNameSubmit(name string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}
	if name == "" || strings.ContainsAny(name, " \t[]@") {
		t.modelAdd = nil
		return t, notice(msgError("invalid provider name (no spaces, brackets, or @)") + "\n")
	}
	t.modelAdd.compatProvider = strings.ToUpper(name)
	return t.openModelAddCompatKey()
}

func (t TUI) runModelAddLocal(instance string) (TUI, tea.Cmd) {
	idx := slices.IndexFunc(config.LocalCompats, func(one config.LocalCompat) bool { return one.Provider == instance })
	if idx < 0 {
		t.modelAdd = nil
		return t, notice(msgError(fmt.Sprintf("unknown local provider %q", instance)) + "\n")
	}
	t.modelAdd.provider = "compat"
	t.modelAdd.compatProvider = instance
	t.modelAdd.compatURL = config.LocalCompats[idx].URL
	return t.openModelAddModelPick()
}

func (t TUI) openModelAddCompatURL() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "URL",
		subtitle: "enter up to /v1 — e.g. http://localhost:11434/v1",
		input:    newPopupInput("", false),
		onConfirm: func(value string) any {
			return ModelAddCompatURLSubmit{url: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) runModelAddCompatURLSubmit(url string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}
	if url == "" {
		t.modelAdd = nil
		return t, notice(msgError("url is required") + "\n")
	}
	url = strings.TrimRight(url, "/")
	if !strings.Contains(url, "://") {
		url = "http://" + url
	}
	t.modelAdd.compatURL = url
	return t.openModelAddCompatName()
}

func (t TUI) openModelAddCompatKey() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:  popupSecret,
		title: "API key (blank to skip)",
		input: newPopupInput("", false),
		onConfirm: func(value string) any {
			return ModelAddCompatKeySubmit{key: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) runModelAddCompatKeySubmit(key string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}
	if key != "" {
		keychainKey := "COMPAT_" + t.modelAdd.compatProvider + "_API_KEY"
		if err := keychain.Set(keychainKey, key); err != nil {
			t.modelAdd = nil
			return t, notice(msgError(fmt.Sprintf("keychain.Set: %v", err)) + "\n")
		}
		if err := config.SaveKey(keychainKey); err != nil {
			t.modelAdd = nil
			return t, notice(msgError(fmt.Sprintf("session.SaveKey: %v", err)) + "\n")
		}
	}
	if err := config.UpsertCompat(t.modelAdd.compatProvider, t.modelAdd.compatURL); err != nil {
		t.modelAdd = nil
		return t, notice(msgError(fmt.Sprintf("UpsertCompat: %v", err)) + "\n")
	}
	return t.openModelAddModelPick()
}

func (t TUI) openModelAddModelPick() (TUI, tea.Cmd) {
	prefix := t.modelAdd.provider + "@"
	if t.modelAdd.provider == "compat" {
		prefix = strings.ToLower(t.modelAdd.compatProvider) + "@"
	}

	cfg, err := config.Load()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("session.Load: %v", err)) + "\n")
	}
	existing := make(map[string]bool, len(cfg.Models))
	for _, m := range cfg.Models {
		existing[m.Name] = true
	}

	if t.modelAdd.provider == "compat" {
		url := t.modelAdd.compatURL
		prov := t.modelAdd.compatProvider
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ids := fetchModelIDs(ctx, url, prov)
			send(CompatModelsResult{ids: ids, windows: modelWindowLabels(ctx, prefix, ids)})
		}()
		return t.openModelAddLoading(prov), nil
	}

	if fn, ok := modelsProviders[t.modelAdd.provider]; ok {
		prov := t.modelAdd.provider
		label := strings.ToUpper(prov[:1]) + prov[1:]
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			config, err := agentkeychain.Config(ctx, prov)
			var ids []string
			if err == nil {
				ids, err = fn(ctx, config, go_llm_router.ModelFilter{TextOnly: true})
			}
			if err != nil {
				slog.Debug("provider Models",
					slog.String("provider", prov),
					slog.Any("error", err))
			}
			send(RemoteModelsResult{ids: ids, windows: modelWindowLabels(ctx, prefix, ids)})
		}()
		return t.openModelAddLoading(label), nil
	}

	t.popup = &Popup{
		kind:  popupText,
		title: fmt.Sprintf("Model name (prefix: %s)", prefix),
		input: newPopupInput("", false),
		onConfirm: func(value string) any {
			name := strings.TrimSpace(value)
			if name == "" {
				return ModelAddModelMultiPick{chosen: ""}
			}
			return ModelAddModelMultiPick{chosen: name + "\x00"}
		},
	}
	return t, nil
}

func (t TUI) openModelAddLoading(label string) TUI {
	t.popup = &Popup{
		kind:        popupMultiSelect,
		title:       fmt.Sprintf("Select %s models (space toggle  enter confirm)", label),
		styledLines: []string{hintStyle.Render("  loading...")},
		multi:       map[int]bool{},
	}
	t.modelAdd.loading = t.popup
	return t
}

func (t TUI) modelAddLoaded() bool {
	return t.modelAdd != nil && t.popup != nil && t.popup == t.modelAdd.loading
}

func (t TUI) runModelAddModelMultiPick(chosen string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, notice(msgError("model add state lost") + "\n")
	}

	prefix := t.modelAdd.provider + "@"
	if t.modelAdd.provider == "compat" {
		prefix = strings.ToLower(t.modelAdd.compatProvider) + "@"
	}

	selected := make(map[string]string)
	if chosen != "" {
		for entry := range strings.SplitSeq(chosen, "\x1F") {
			parts := strings.SplitN(entry, "\x00", 2)
			name := parts[0]
			desc := ""
			if len(parts) > 1 {
				desc = parts[1]
			}
			selected[prefix+name] = desc
		}
	}

	cfg, err := config.Load()
	if err != nil {
		t.modelAdd = nil
		return t, notice(msgError(fmt.Sprintf("session.Load: %v", err)) + "\n")
	}

	var kept []config.ModelEntry
	var removed []string
	for _, m := range cfg.Models {
		if strings.HasPrefix(m.Name, prefix) {
			if _, ok := selected[m.Name]; ok {
				kept = append(kept, config.ModelEntry{Name: m.Name})
				delete(selected, m.Name)
			} else {
				removed = append(removed, m.Name)
			}
		} else {
			kept = append(kept, m)
		}
	}

	var added []string
	for fullName := range selected {
		kept = append(kept, config.ModelEntry{Name: fullName})
		added = append(added, fullName)
	}
	slices.Sort(added)

	cfg.Models = kept
	if cfg.DispatcherModel != "" && strings.HasPrefix(cfg.DispatcherModel, prefix) {
		found := false
		for _, m := range kept {
			if m.Name == cfg.DispatcherModel {
				found = true
				break
			}
		}
		if !found {
			cfg.DispatcherModel = ""
		}
	}
	if cfg.DispatcherModel == "" && len(kept) > 0 {
		cfg.DispatcherModel = kept[0].Name
	}

	if err := config.Save(cfg); err != nil {
		t.modelAdd = nil
		return t, notice(msgError(fmt.Sprintf("session.Save: %v", err)) + "\n")
	}

	agents.Reload()

	var summary []string
	if len(added) > 0 {
		summary = append(summary, fmt.Sprintf("added: %s", strings.Join(added, ", ")))
	}
	if len(removed) > 0 {
		summary = append(summary, fmt.Sprintf("removed: %s", strings.Join(removed, ", ")))
	}

	t.modelAdd = nil
	if len(summary) == 0 {
		return t, nil
	}
	slog.Debug("models updated", slog.String("summary", strings.Join(summary, "  ")))
	return t, nil
}

func (t TUI) runCompatModelsResult(msg CompatModelsResult) (TUI, tea.Cmd) {
	if !t.modelAddLoaded() {
		return t, nil
	}

	prefix := strings.ToLower(t.modelAdd.compatProvider) + "@"

	if len(msg.ids) == 0 {
		t.popup.styledLines = []string{warnStyle.Render("  no models found at endpoint")}
		return t, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("config.Load: %v", err)) + "\n")
	}
	existing := make(map[string]bool, len(cfg.Models))
	for _, m := range cfg.Models {
		existing[m.Name] = true
	}

	sort.Strings(msg.ids)

	options := make([]string, len(msg.ids))
	values := make([]string, len(msg.ids))
	preSelected := make(map[int]bool)

	for i, id := range msg.ids {
		fullName := prefix + id
		label := id
		if window := msg.windows[id]; window != "" {
			label += "  " + window
		}
		if existing[fullName] {
			preSelected[i] = true
		}
		options[i] = label
		values[i] = id + "\x00"
	}

	t.popup = &Popup{
		kind:       popupMultiSelect,
		title:      fmt.Sprintf("Select %s models (space toggle  enter confirm)", t.modelAdd.compatProvider),
		options:    options,
		values:     values,
		multi:      preSelected,
		allOptions: options,
		allValues:  values,
		searchable: true,
		input:      newModelSearchInput(),
		onConfirm: func(chosen string) any {
			return ModelAddModelMultiPick{chosen: chosen}
		},
	}
	return t, nil
}

var modelsProviders = map[string]func(context.Context, go_llm_router.Config, go_llm_router.ModelFilter) ([]string, error){
	"codex":        openaicodex.Models,
	"grok-oauth":   grokoauth.Models,
	"copilot":      copilot.Models,
	"cloudflare":   cloudflare.Models,
	"openai":       openai.Models,
	"claude":       claude.Models,
	"gemini":       gemini.Models,
	"ollama-cloud": ollamacloud.Models,
	"grok":         grok.Models,
	"deepseek":     deepseek.Models,
	"mistral":      mistral.Models,
	"nvidia":       nvidia.Models,
	"openrouter":   openrouter.Models,
	"claude-code": func(ctx context.Context, cfg go_llm_router.Config, _ go_llm_router.ModelFilter) ([]string, error) {
		return claudeCode.Models(ctx, cfg)
	},
}

func (t TUI) runRemoteModelsResult(msg RemoteModelsResult) (TUI, tea.Cmd) {
	if !t.modelAddLoaded() {
		return t, nil
	}

	prefix := t.modelAdd.provider + "@"

	if len(msg.ids) == 0 {
		t.popup.styledLines = []string{warnStyle.Render("  no models found at endpoint")}
		return t, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("config.Load: %v", err)) + "\n")
	}
	existing := make(map[string]bool, len(cfg.Models))
	for _, m := range cfg.Models {
		existing[m.Name] = true
	}

	sort.Strings(msg.ids)

	options := make([]string, len(msg.ids))
	values := make([]string, len(msg.ids))
	preSelected := make(map[int]bool)

	for i, id := range msg.ids {
		fullName := prefix + id
		label := id
		if window := msg.windows[id]; window != "" {
			label += "  " + window
		}
		if existing[fullName] {
			preSelected[i] = true
		}
		options[i] = label
		values[i] = id + "\x00"
	}

	t.popup = &Popup{
		kind:       popupMultiSelect,
		title:      fmt.Sprintf("Select %s models (space toggle  enter confirm)", t.modelAdd.provider),
		options:    options,
		values:     values,
		multi:      preSelected,
		allOptions: options,
		allValues:  values,
		searchable: true,
		input:      newModelSearchInput(),
		onConfirm: func(chosen string) any {
			return ModelAddModelMultiPick{chosen: chosen}
		},
	}
	return t, nil
}

func fetchModelIDs(ctx context.Context, baseURL, provider string) []string {
	ids, err := probe.CompatModels(ctx, baseURL, keychain.Get("COMPAT_"+strings.ToUpper(provider)+"_API_KEY"))
	if err != nil {
		return nil
	}
	return ids
}

func modelWindowLabels(ctx context.Context, prefix string, ids []string) map[string]string {
	dic := make(map[string]string, len(ids))
	for _, id := range ids {
		in, out, ok := compact.Window(ctx, prefix+id)
		if !ok {
			continue
		}
		var parts []string
		if in > 0 {
			parts = append(parts, windowTokenText(in))
		}
		if out > 0 {
			parts = append(parts, windowTokenText(out))
		}
		dic[id] = strings.Join(parts, "/")
	}
	return dic
}

func windowTokenText(value int) string {
	switch {
	case value >= 1_000_000:
		return fmt.Sprintf("%dM", int(math.Round(float64(value)/1_000_000)))
	case value >= 1_000:
		return fmt.Sprintf("%dK", int(math.Round(float64(value)/1_000)))
	default:
		return strconv.Itoa(value)
	}
}

func newModelSearchInput() textarea.Model {
	input := newPopupInput("", false)
	input.Placeholder = "Search models..."
	input.SetPromptFunc(2, func(int) string {
		return hintStyle.Render("/ ")
	})
	return input
}
