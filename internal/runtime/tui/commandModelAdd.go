package tui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/copilot"
	openaicodex "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex"
	"github.com/pardnchiu/agenvoy/internal/runtime/kuradb"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

type modelAddItem struct {
	provider       string
	compatProvider string
	compatURL      string
}

type ModelAddProviderPick struct{ provider string }
type ModelAddAPIKeyReplace struct{ replace string }
type ModelAddAPIKeySubmit struct{ key string }
type ModelAddCompatNameSubmit struct{ name string }
type ModelAddCompatURLSubmit struct{ url string }
type ModelAddCompatKeySubmit struct{ key string }
type ModelAddModelPick struct{ name, description string }
type ModelAddDone struct{ err error }

type OAuthInfo struct {
	url      string
	userCode string
}
type OAuthSuccess struct{ provider string }
type OAuthFailed struct{ err error }
type OAuthReLoginPick struct{ replace string }

var modelAddProviders = []struct {
	name  string
	label string
}{
	{"copilot", "Github Copilot"},
	{"openai", "OpenAI"},
	{"codex", "Codex (OpenAI Subscription)"},
	{"claude", "Claude"},
	{"gemini", "Gemini"},
	{"grok", "Grok"},
	{"deepseek", "DeepSeek"},
	{"nvidia", "NVIDIA NIM"},
	{"compat", "Compatibility (custom endpoint)"},
}

func (t TUI) commandModelAdd() (TUI, tea.Cmd, bool) {
	t.modelAdd = &modelAddItem{}
	options := make([]string, len(modelAddProviders))
	values := make([]string, len(modelAddProviders))
	for i, p := range modelAddProviders {
		options[i] = p.label
		values[i] = p.name
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Model · global add · provider",
		options: options,
		values:  values,
		onConfirm: func(chosen string) any {
			return ModelAddProviderPick{provider: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) runModelAddProviderPick(name string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		t.modelAdd = &modelAddItem{}
	}
	t.modelAdd.provider = name
	switch name {
	case "copilot", "codex":
		return t.modelAddViaOAuth()
	case "compat":
		return t.openModelAddCompatName()
	default:
		return t.openModelAddAPIKey()
	}
}

func (t TUI) modelAddViaOAuth() (TUI, tea.Cmd) {
	prov := t.modelAdd.provider
	hasToken := false
	switch prov {
	case "copilot":
		hasToken = copilot.HasToken()
	case "codex":
		hasToken = openaicodex.HasToken()
	}
	if hasToken {
		label := strings.ToUpper(prov[:1]) + prov[1:]
		t.popup = &Popup{
			kind:    popupSingleSelect,
			title:   fmt.Sprintf("%s token exists · re-login?", label),
			options: []string{"No (keep existing)", "Yes (re-authenticate)"},
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
		title:    fmt.Sprintf("%s OAuth · waiting for device code…", strings.ToUpper(prov[:1])+prov[1:]),
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
		return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
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
		if copilot.HasToken() {
			if cerr := copilot.ClearToken(); cerr != nil {
				send(OAuthFailed{err: fmt.Errorf("ClearToken: %w", cerr)})
				return
			}
		}
		err = copilot.AuthWithCallback(ctx, func(code *copilot.DeviceCode) {
			send(OAuthInfo{
				url:      code.VerificationURI,
				userCode: code.UserCode,
			})
			_ = openBrowser(code.VerificationURI)
		})
	case "codex":
		if openaicodex.HasToken() {
			if cerr := openaicodex.ClearToken(); cerr != nil {
				send(OAuthFailed{err: fmt.Errorf("ClearToken: %w", cerr)})
				return
			}
		}
		err = openaicodex.AuthWithCallback(ctx, func(url string) {
			send(OAuthInfo{url: url})
			_ = openBrowser(url)
		})
	default:
		err = fmt.Errorf("unsupported oauth provider: %s", prov)
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
	title := "OAuth · open browser to authorize"
	if t.popup.oauth.provider != "" {
		label := strings.ToUpper(t.popup.oauth.provider[:1]) + t.popup.oauth.provider[1:]
		title = fmt.Sprintf("%s OAuth · open browser to authorize", label)
	}
	t.popup.title = title
	t.popup.subtitle = ""
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
		return t, tea.Println(hintStyle.Render("⎯ oauth cancelled") + "\n")
	case errors.Is(msg.err, context.DeadlineExceeded):
		return t, tea.Println(warnStyle.Render("⎯ oauth timed out · device code expired") + "\n")
	}
	return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] oauth: %v", msg.err)) + "\n")
}

func (t TUI) openModelAddAPIKey() (TUI, tea.Cmd) {
	envKey := strings.ToUpper(t.modelAdd.provider) + "_API_KEY"
	label := strings.ToUpper(t.modelAdd.provider[:1]) + t.modelAdd.provider[1:]
	if keychain.Get(envKey) != "" {
		t.popup = &Popup{
			kind:    popupSingleSelect,
			title:   fmt.Sprintf("%s API key exists · replace?", label),
			options: []string{"No (keep existing)", "Yes (overwrite)"},
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
		onConfirm: func(value string) any {
			return ModelAddAPIKeySubmit{key: value}
		},
	}
	return t, nil
}

func (t TUI) runModelAddAPIKeyReplace(replace string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
	}
	if replace == "yes" {
		label := strings.ToUpper(t.modelAdd.provider[:1]) + t.modelAdd.provider[1:]
		t.popup = &Popup{
			kind:  popupSecret,
			title: fmt.Sprintf("%s API key (new)", label),
			onConfirm: func(value string) any {
				return ModelAddAPIKeySubmit{key: value}
			},
		}
		return t, nil
	}
	return t.openModelAddModelPick()
}

func (t TUI) runModelAddAPIKeySubmit(key string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		t.modelAdd = nil
		return t, tea.Println(errorStyle.Render("[!] api key required") + "\n")
	}
	envKey := strings.ToUpper(t.modelAdd.provider) + "_API_KEY"
	if err := keychain.Set(envKey, key); err != nil {
		t.modelAdd = nil
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] keychain.Set: %v", err)) + "\n")
	}
	if envKey == "OPENAI_API_KEY" {
		if err := kuradb.SyncOpenAIKey(key); err != nil {
			t.modelAdd = nil
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] kuradb SyncOpenAIKey: %v", err)) + "\n")
		}
	}
	if err := config.SaveKey(envKey); err != nil {
		t.modelAdd = nil
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.SaveKey: %v", err)) + "\n")
	}
	return t.openModelAddModelPick()
}

func (t TUI) openModelAddCompatName() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:  popupText,
		title: "Compat provider name (ex. ollama)",
		onConfirm: func(value string) any {
			return ModelAddCompatNameSubmit{name: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) runModelAddCompatNameSubmit(name string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
	}
	if name == "" || strings.ContainsAny(name, " \t[]@") {
		t.modelAdd = nil
		return t, tea.Println(errorStyle.Render("[!] invalid provider name (no spaces, brackets, or @)") + "\n")
	}
	t.modelAdd.compatProvider = strings.ToUpper(name)
	return t.openModelAddCompatURL()
}

func (t TUI) openModelAddCompatURL() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:  popupText,
		title: "URL (blank = http://localhost:11434/v1)",
		onConfirm: func(value string) any {
			return ModelAddCompatURLSubmit{url: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) runModelAddCompatURLSubmit(url string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
	}
	if url == "" {
		url = "http://localhost:11434/v1"
	}
	url = strings.TrimRight(url, "/")
	if err := config.UpsertCompat(t.modelAdd.compatProvider, url); err != nil {
		t.modelAdd = nil
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] UpsertCompat: %v", err)) + "\n")
	}
	t.modelAdd.compatURL = url
	return t.openModelAddCompatKey()
}

func (t TUI) openModelAddCompatKey() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:  popupSecret,
		title: "API key (blank to skip)",
		onConfirm: func(value string) any {
			return ModelAddCompatKeySubmit{key: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) runModelAddCompatKeySubmit(key string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
	}
	if key != "" {
		keychainKey := "COMPAT_" + t.modelAdd.compatProvider + "_API_KEY"
		if err := keychain.Set(keychainKey, key); err != nil {
			t.modelAdd = nil
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] keychain.Set: %v", err)) + "\n")
		}
		if err := config.SaveKey(keychainKey); err != nil {
			t.modelAdd = nil
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.SaveKey: %v", err)) + "\n")
		}
	}
	return t.openModelAddModelPick()
}

func (t TUI) openModelAddModelPick() (TUI, tea.Cmd) {
	prefix := t.modelAdd.provider + "@"
	if t.modelAdd.provider == "compat" {
		prefix = fmt.Sprintf("compat[%s]@", t.modelAdd.compatProvider)
	}

	var models map[string]provider.ModelItem
	if t.modelAdd.provider != "compat" {
		models = provider.Models(t.modelAdd.provider)
	}

	if len(models) == 0 {
		t.popup = &Popup{
			kind:  popupText,
			title: fmt.Sprintf("Model name (prefix: %s)", prefix),
			onConfirm: func(value string) any {
				return ModelAddModelPick{name: strings.TrimSpace(value)}
			},
		}
		return t, nil
	}

	names := make([]string, 0, len(models))
	for n := range models {
		names = append(names, n)
	}
	sort.SliceStable(names, func(i, j int) bool {
		pi := strings.Contains(names[i], "-preview") || strings.Contains(names[i], "-experimental")
		pj := strings.Contains(names[j], "-preview") || strings.Contains(names[j], "-experimental")
		if pi != pj {
			return !pi
		}
		return names[i] < names[j]
	})

	options := make([]string, len(names))
	values := make([]string, len(names))
	for i, n := range names {
		desc := models[n].Description
		if desc != "" {
			options[i] = fmt.Sprintf("%s  %s", n, hintStyle.Render(desc))
		} else {
			options[i] = n
		}
		values[i] = n + "\x00" + desc
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   fmt.Sprintf("Select %s model", t.modelAdd.provider),
		options: options,
		values:  values,
		onConfirm: func(chosen string) any {
			parts := strings.SplitN(chosen, "\x00", 2)
			name := parts[0]
			desc := ""
			if len(parts) > 1 {
				desc = parts[1]
			}
			return ModelAddModelPick{name: name, description: desc}
		},
	}
	return t, nil
}

func (t TUI) runModelAddModelPick(name, description string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		t.modelAdd = nil
		return t, tea.Println(errorStyle.Render("[!] model name required") + "\n")
	}
	prefix := t.modelAdd.provider + "@"
	if t.modelAdd.provider == "compat" {
		prefix = fmt.Sprintf("compat[%s]@", t.modelAdd.compatProvider)
	}
	fullName := prefix + name
	t.modelAdd = nil

	cfg, err := config.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n")
	}

	found := false
	for i, m := range cfg.Models {
		if m.Name == fullName {
			cfg.Models[i].Description = description
			found = true
			break
		}
	}
	if !found {
		cfg.Models = append(cfg.Models, config.ModelEntry{Name: fullName, Description: description})
	}
	if cfg.DispatcherModel == "" {
		cfg.DispatcherModel = fullName
	}
	if err := config.Save(cfg); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Save: %v", err)) + "\n")
	}

	agents.Reload()
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ added: %s · registry reloaded", fullName)) + "\n")
}
