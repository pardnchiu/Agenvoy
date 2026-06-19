package tui

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

type ModelScanLocalResult struct {
	entries []localModelEntry
	err     error
}

type ModelScanLocalPick struct{ chosen string }

type localModelEntry struct {
	id       string
	provider string
	baseURL  string
}

type modelsResponse struct {
	Data []struct {
		ID                 string `json:"id"`
		ModelPickerEnabled bool   `json:"model_picker_enabled"`
		Policy             struct {
			State string `json:"state"`
		} `json:"policy"`
	} `json:"data"`
}

var defaultLocalEndpoints = []struct {
	provider string
	url      string
}{
	{"OLLAMA", "http://localhost:11434/v1"},
	{"LMSTUDIO", "http://localhost:1234/v1"},
	{"LLAMACPP", "http://localhost:8080/v1"},
}


func scanLocalModels(ctx context.Context, extraURL string) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	targets := buildScanTargets(extraURL)

	type result struct {
		provider string
		baseURL  string
		ids      []string
	}

	ch := make(chan result, len(targets))
	for _, t := range targets {
		go func(provider, baseURL string) {
			ids := fetchModelIDs(ctx, baseURL, provider)
			ch <- result{provider: provider, baseURL: baseURL, ids: ids}
		}(t.provider, t.baseURL)
	}

	var entries []localModelEntry
	for range targets {
		r := <-ch
		for _, id := range r.ids {
			entries = append(entries, localModelEntry{
				id:       id,
				provider: r.provider,
				baseURL:  r.baseURL,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].provider != entries[j].provider {
			return entries[i].provider < entries[j].provider
		}
		return entries[i].id < entries[j].id
	})

	send(ModelScanLocalResult{entries: entries})
}

func buildScanTargets(extraURL string) []struct {
	provider string
	baseURL  string
} {
	seen := make(map[string]bool)
	var targets []struct {
		provider string
		baseURL  string
	}

	cfg, _ := config.Load()
	if cfg != nil {
		for _, c := range cfg.Compats {
			url := strings.TrimRight(c.URL, "/")
			norm := strings.ToLower(url)
			if seen[norm] {
				continue
			}
			seen[norm] = true
			targets = append(targets, struct {
				provider string
				baseURL  string
			}{c.Provider, url})
		}
	}

	for _, d := range defaultLocalEndpoints {
		norm := strings.ToLower(d.url)
		if seen[norm] {
			continue
		}
		seen[norm] = true
		targets = append(targets, struct {
			provider string
			baseURL  string
		}{d.provider, d.url})
	}

	if extraURL != "" {
		extraURL = strings.TrimRight(extraURL, "/")
		if !strings.Contains(extraURL, "://") {
			extraURL = "http://" + extraURL
		}
		norm := strings.ToLower(extraURL)
		if !seen[norm] {
			targets = append(targets, struct {
				provider string
				baseURL  string
			}{"CUSTOM", extraURL})
		}
	}

	return targets
}

func fetchModelIDs(ctx context.Context, baseURL, provider string) []string {
	endpoint := baseURL + "/models"

	var headers map[string]string
	apiKeyEnvKey := "COMPAT_" + strings.ToUpper(provider) + "_API_KEY"
	if key := keychain.Get(apiKeyEnvKey); key != "" {
		headers = map[string]string{
			"Authorization": "Bearer " + key,
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	data, status, err := go_pkg_http.GET[modelsResponse](ctx, client, endpoint, headers)
	if err != nil || status != http.StatusOK {
		return nil
	}

	ids := make([]string, 0, len(data.Data))
	for _, m := range data.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func (t TUI) runModelScanLocalResult(msg ModelScanLocalResult) (TUI, tea.Cmd) {
	if msg.err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] search local: %v", msg.err)) + "\n")
	}
	if len(msg.entries) == 0 {
		return t, tea.Println(hintStyle.Render("⎯ no local models found") + "\n")
	}

	cfg, err := config.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] config.Load: %v", err)) + "\n")
	}
	existing := make(map[string]bool, len(cfg.Models))
	for _, m := range cfg.Models {
		existing[m.Name] = true
	}

	options := make([]string, len(msg.entries))
	values := make([]string, len(msg.entries))
	preSelected := make(map[int]bool)

	for i, e := range msg.entries {
		fullName := fmt.Sprintf("compat[%s]@%s", e.provider, e.id)
		label := fmt.Sprintf("%s · %s", e.id, e.provider)
		if existing[fullName] {
			label += " · (added)"
			preSelected[i] = true
		}
		options[i] = label
		values[i] = fullName + "\x00" + e.provider + "\x00" + e.baseURL
	}

	t.popup = &Popup{
		kind:    popupMultiSelect,
		title:   "Local models (space toggle · enter confirm)",
		options: options,
		values:  values,
		multi:   preSelected,
		onConfirm: func(chosen string) any {
			return ModelScanLocalPick{chosen: chosen}
		},
	}
	return t, nil
}

func (t TUI) runModelScanLocalPick(chosen string) (TUI, tea.Cmd) {
	if chosen == "" {
		return t, tea.Println(hintStyle.Render("⎯ no models selected") + "\n")
	}

	cfg, err := config.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] config.Load: %v", err)) + "\n")
	}

	existing := make(map[string]bool, len(cfg.Models))
	for _, m := range cfg.Models {
		existing[m.Name] = true
	}

	compatURLs := make(map[string]string, len(cfg.Compats))
	for _, c := range cfg.Compats {
		compatURLs[strings.ToUpper(c.Provider)] = c.URL
	}

	var added []string
	for _, entry := range strings.Split(chosen, "\x1F") {
		parts := strings.SplitN(entry, "\x00", 3)
		fullName := strings.TrimSpace(parts[0])
		if fullName == "" || existing[fullName] {
			continue
		}

		cfg.Models = append(cfg.Models, config.ModelEntry{Name: fullName})
		added = append(added, fullName)

		if len(parts) < 3 {
			continue
		}
		provider := strings.ToUpper(parts[1])
		baseURL := parts[2]
		if provider == "" || baseURL == "" {
			continue
		}
		if _, ok := compatURLs[provider]; ok {
			continue
		}
		cfg.Compats = append(cfg.Compats, config.CompatEntry{
			Provider: provider,
			URL:      baseURL,
		})
		compatURLs[provider] = baseURL
	}

	if len(added) == 0 {
		return t, tea.Println(hintStyle.Render("⎯ no new models added") + "\n")
	}

	if cfg.DispatcherModel == "" {
		cfg.DispatcherModel = cfg.Models[0].Name
	}

	if err := config.Save(cfg); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] config.Save: %v", err)) + "\n")
	}

	agents.Reload()

	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ added: %s · registry reloaded", strings.Join(added, ", "))) + "\n")
}

