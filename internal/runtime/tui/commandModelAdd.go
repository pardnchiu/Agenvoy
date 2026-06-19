package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/copilot"
	grokoauth "github.com/pardnchiu/agenvoy/internal/agents/provider/grokOauth"
	openaicodex "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex"
	"github.com/pardnchiu/agenvoy/internal/runtime/kuradb"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"
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
type ModelAddModelMultiPick struct{ chosen string }
type ModelAddDone struct{ err error }
type CompatModelsResult struct{ ids []string }
type RemoteModelsResult struct{ ids []string }
type OpenAIMethodPick struct{ method string }
type GrokMethodPick struct{ method string }

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
	{"openai", "OpenAI          API key or Codex subscription"},
	{"claude", "Claude          API key"},
	{"gemini", "Gemini          API key"},
	{"grok", "Grok            API key or xAI subscription"},
	{"copilot", "Github Copilot  GitHub subscription"},
	{"deepseek", "DeepSeek        API key"},
	{"nvidia", "NVIDIA NIM      API key"},
	{"openrouter", "OpenRouter      API key"},
	{"compat", "Local/Custom    Ollama, LM Studio, or custom URL"},
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
	case "openai":
		t.popup = &Popup{
			kind:    popupSingleSelect,
			title:   "OpenAI · method",
			options: []string{"API Key  pay per token", "Codex    subscription"},
			values:  []string{"api-key", "codex"},
			onConfirm: func(chosen string) any {
				return OpenAIMethodPick{method: chosen}
			},
		}
		return t, nil
	case "grok":
		t.popup = &Popup{
			kind:    popupSingleSelect,
			title:   "Grok · method",
			options: []string{"API Key  pay per token", "xAI      subscription"},
			values:  []string{"api-key", "grok-oauth"},
			onConfirm: func(chosen string) any {
				return GrokMethodPick{method: chosen}
			},
		}
		return t, nil
	case "copilot", "codex", "grok-oauth":
		return t.modelAddViaOAuth()
	case "compat":
		return t.openModelAddCompatURL()
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
	case "grok-oauth":
		hasToken = grokoauth.HasToken()
	}
	if hasToken {
		label := strings.ToUpper(prov[:1]) + prov[1:]
		t.popup = &Popup{
			kind:    popupSingleSelect,
			title:   fmt.Sprintf("%s token exists · re-login?", label),
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
		})
	case "grok-oauth":
		if grokoauth.HasToken() {
			if cerr := grokoauth.ClearToken(); cerr != nil {
				send(OAuthFailed{err: fmt.Errorf("ClearToken: %w", cerr)})
				return
			}
		}
		err = grokoauth.AuthWithCallback(ctx, func(url string) {
			send(OAuthInfo{url: url})
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
	if msg.url != "" && msg.userCode == "" {
		openBrowser(msg.url)
	}
	return t, nil
}

func (t TUI) runOAuthSuccess(msg OAuthSuccess) (TUI, tea.Cmd) {
	if t.enableImage2AfterOAuth && msg.provider == "codex" {
		t.enableImage2AfterOAuth = false
		t.modelAdd = nil
		t.popup = nil
		return t, setImage2("enable")
	}
	if t.modelAdd == nil {
		t.modelAdd = &modelAddItem{provider: msg.provider}
	}
	t.popup = nil
	return t.openModelAddModelPick()
}

func (t TUI) runOAuthFailed(msg OAuthFailed) (TUI, tea.Cmd) {
	t.modelAdd = nil
	t.enableImage2AfterOAuth = false
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
	return t.openModelAddCompatKey()
}

func (t TUI) openModelAddCompatURL() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "URL (blank = scan local)",
		subtitle: "enter up to /v1 — e.g. http://localhost:11434/v1",
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
		go scanLocalModels(t.ctx, "")
		return t, nil
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
	if err := config.UpsertCompat(t.modelAdd.compatProvider, t.modelAdd.compatURL); err != nil {
		t.modelAdd = nil
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] UpsertCompat: %v", err)) + "\n")
	}
	return t.openModelAddModelPick()
}

func (t TUI) openModelAddModelPick() (TUI, tea.Cmd) {
	prefix := t.modelAdd.provider + "@"
	if t.modelAdd.provider == "compat" {
		prefix = fmt.Sprintf("compat[%s]@", t.modelAdd.compatProvider)
	}

	cfg, err := config.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n")
	}
	existing := make(map[string]bool, len(cfg.Models))
	for _, m := range cfg.Models {
		existing[m.Name] = true
	}

	if t.modelAdd.provider == "compat" {
		url := t.modelAdd.compatURL
		prov := t.modelAdd.compatProvider
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ids := fetchModelIDs(ctx, url, prov)
			send(CompatModelsResult{ids: ids})
		}()
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ %s · fetching models…", prov)) + "\n")
	}

	if endpoint, headers := remoteModelsEndpoint(t.modelAdd.provider); endpoint != "" {
		prov := t.modelAdd.provider
		label := strings.ToUpper(prov[:1]) + prov[1:]
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ids := fetchRemoteModelIDs(ctx, endpoint, headers, prov)
			send(RemoteModelsResult{ids: ids})
		}()
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ %s · fetching models…", label)) + "\n")
	}

	t.popup = &Popup{
		kind:  popupText,
		title: fmt.Sprintf("Model name (prefix: %s)", prefix),
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

func (t TUI) runModelAddModelMultiPick(chosen string) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
	}

	prefix := t.modelAdd.provider + "@"
	if t.modelAdd.provider == "compat" {
		prefix = fmt.Sprintf("compat[%s]@", t.modelAdd.compatProvider)
	}

	selected := make(map[string]string)
	if chosen != "" {
		for _, entry := range strings.Split(chosen, "\x1F") {
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
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n")
	}

	var kept []config.ModelEntry
	var removed []string
	for _, m := range cfg.Models {
		if strings.HasPrefix(m.Name, prefix) {
			if desc, ok := selected[m.Name]; ok {
				kept = append(kept, config.ModelEntry{Name: m.Name, Description: desc})
				delete(selected, m.Name)
			} else {
				removed = append(removed, m.Name)
			}
		} else {
			kept = append(kept, m)
		}
	}

	var added []string
	for fullName, desc := range selected {
		kept = append(kept, config.ModelEntry{Name: fullName, Description: desc})
		added = append(added, fullName)
	}
	sort.Slice(added, func(i, j int) bool { return added[i] < added[j] })

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
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Save: %v", err)) + "\n")
	}

	agents.Reload()

	var summary []string
	if len(added) > 0 {
		summary = append(summary, fmt.Sprintf("added: %s", strings.Join(added, ", ")))
	}
	if len(removed) > 0 {
		summary = append(summary, fmt.Sprintf("removed: %s", strings.Join(removed, ", ")))
	}

	providerName := t.modelAdd.provider
	t.modelAdd = nil
	if len(summary) == 0 {
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ %s models unchanged", providerName)) + "\n")
	}
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ %s · registry reloaded", strings.Join(summary, " · "))) + "\n")
}

func (t TUI) runCompatModelsResult(msg CompatModelsResult) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
	}

	prefix := fmt.Sprintf("compat[%s]@", t.modelAdd.compatProvider)

	if len(msg.ids) == 0 {
		t.modelAdd = nil
		return t, tea.Println(warnStyle.Render(fmt.Sprintf("⎯ %s · no models found at endpoint", prefix)) + "\n")
	}

	cfg, err := config.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] config.Load: %v", err)) + "\n")
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
		if existing[fullName] {
			label += " · (added)"
			preSelected[i] = true
		}
		options[i] = label
		values[i] = id + "\x00"
	}

	t.popup = &Popup{
		kind:    popupMultiSelect,
		title:   fmt.Sprintf("Select %s models (space toggle · enter confirm)", t.modelAdd.compatProvider),
		options: options,
		values:  values,
		multi:   preSelected,
		onConfirm: func(chosen string) any {
			return ModelAddModelMultiPick{chosen: chosen}
		},
	}
	return t, nil
}

var remoteModelsProviders = map[string]struct {
	endpoint   string
	keychainKey string
}{
	"codex":      {"https://agenvoy-codex.pardn.workers.dev/models", ""},
	"openai":     {"https://api.openai.com/v1/models", "OPENAI_API_KEY"},
	"claude":     {"https://api.anthropic.com/v1/models", "CLAUDE_API_KEY"},
	"gemini":     {"https://generativelanguage.googleapis.com/v1beta/models", "GEMINI_API_KEY"},
	"grok":       {"https://api.x.ai/v1/models", "GROK_API_KEY"},
	"deepseek":   {"https://api.deepseek.com/v1/models", "DEEPSEEK_API_KEY"},
	"nvidia":     {"https://integrate.api.nvidia.com/v1/models", "NVIDIA_API_KEY"},
	"openrouter": {"https://openrouter.ai/api/v1/models", "OPENROUTER_API_KEY"},
}

func remoteModelsEndpoint(prov string) (string, map[string]string) {
	if prov == "grok-oauth" {
		raw := keychain.Get("agenvoy.grok-oauth.token")
		if raw == "" {
			return "", nil
		}
		var token struct {
			AccessToken string `json:"access_token"`
		}
		if json.Unmarshal([]byte(raw), &token) != nil || token.AccessToken == "" {
			return "", nil
		}
		return "https://api.x.ai/v1/models", map[string]string{
			"Authorization": "Bearer " + token.AccessToken,
		}
	}

	if prov == "copilot" {
		raw := keychain.Get("agenvoy.copilot.token")
		if raw == "" {
			return "", nil
		}
		var ghToken struct {
			AccessToken string `json:"access_token"`
		}
		if json.Unmarshal([]byte(raw), &ghToken) != nil || ghToken.AccessToken == "" {
			return "", nil
		}
		client := &http.Client{Timeout: 10 * time.Second}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		type refreshResp struct {
			Token string `json:"token"`
		}
		refresh, status, err := go_pkg_http.GET[refreshResp](ctx, client, "https://api.github.com/copilot_internal/v2/token", map[string]string{
			"Authorization":  "token " + ghToken.AccessToken,
			"Accept":         "application/json",
			"Editor-Version": "vscode/1.95.0",
		})
		if err != nil || status != http.StatusOK || refresh.Token == "" {
			return "", nil
		}
		return "https://api.githubcopilot.com/models", map[string]string{
			"Authorization":  "Bearer " + refresh.Token,
			"Editor-Version": "vscode/1.95.0",
		}
	}

	info, ok := remoteModelsProviders[prov]
	if !ok {
		return "", nil
	}
	if info.keychainKey == "" {
		return info.endpoint, nil
	}
	apiKey := keychain.Get(info.keychainKey)
	if apiKey == "" {
		return "", nil
	}
	if prov == "claude" {
		return info.endpoint, map[string]string{
			"x-api-key":         apiKey,
			"anthropic-version": "2023-06-01",
		}
	}
	if prov == "gemini" {
		return info.endpoint + "?key=" + apiKey, nil
	}
	return info.endpoint, map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
}

type geminiModelsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func fetchRemoteModelIDs(ctx context.Context, endpoint string, headers map[string]string, prov string) []string {
	client := &http.Client{Timeout: 10 * time.Second}

	if prov == "gemini" {
		data, status, err := go_pkg_http.GET[geminiModelsResponse](ctx, client, endpoint, headers)
		if err != nil || status != http.StatusOK {
			return nil
		}
		ids := make([]string, 0, len(data.Models))
		for _, m := range data.Models {
			name := strings.TrimPrefix(m.Name, "models/")
			if name != "" {
				ids = append(ids, name)
			}
		}
		return ids
	}

	data, status, err := go_pkg_http.GET[modelsResponse](ctx, client, endpoint, headers)
	if err != nil || status != http.StatusOK {
		return nil
	}
	ids := make([]string, 0, len(data.Data))
	for _, m := range data.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if prov == "copilot" && (!m.ModelPickerEnabled || m.Policy.State == "disabled") {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func (t TUI) runRemoteModelsResult(msg RemoteModelsResult) (TUI, tea.Cmd) {
	if t.modelAdd == nil {
		return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
	}

	prefix := t.modelAdd.provider + "@"

	if len(msg.ids) == 0 {
		prov := t.modelAdd.provider
		t.modelAdd = nil
		return t, tea.Println(warnStyle.Render(fmt.Sprintf("⎯ %s · no models found at endpoint", prov)) + "\n")
	}

	cfg, err := config.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] config.Load: %v", err)) + "\n")
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
		if existing[fullName] {
			label += " · (added)"
			preSelected[i] = true
		}
		options[i] = label
		values[i] = id + "\x00"
	}

	t.popup = &Popup{
		kind:    popupMultiSelect,
		title:   fmt.Sprintf("Select %s models (space toggle · enter confirm)", t.modelAdd.provider),
		options: options,
		values:  values,
		multi:   preSelected,
		onConfirm: func(chosen string) any {
			return ModelAddModelMultiPick{chosen: chosen}
		},
	}
	return t, nil
}
