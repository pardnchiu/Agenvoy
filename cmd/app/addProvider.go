package main

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/copilot"
	openaicodex "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/go-utils/filesystem/keychain"
	"golang.org/x/term"
)

type Provider struct {
	Prefix string
}

func (p Provider) name() string {
	name := strings.TrimSuffix(p.Prefix, "@")
	if idx := strings.Index(name, "["); idx != -1 {
		name = name[:idx]
	}
	return name
}

func (p Provider) label() string {
	n := p.name()
	if n == "" {
		return p.Prefix
	}
	return strings.ToUpper(n[:1]) + n[1:]
}

func (p Provider) envKey() string {
	return strings.ToUpper(p.name()) + "_API_KEY"
}

var providers = []Provider{
	{Prefix: "copilot@"},
	{Prefix: "openai@"},
	{Prefix: "codex@"},
	{Prefix: "claude@"},
	{Prefix: "gemini@"},
	{Prefix: "nvidia@"},
	{Prefix: "compat"},
}

func runAdd() {
	items := make([]string, len(providers)+1)
	for i, p := range providers {
		items[i] = p.label()
	}
	items[len(providers)] = "exit"

	selector := promptui.Select{
		Label:        "Select provider to add",
		Items:        items,
		HideSelected: true,
	}
	index, _, err := selector.Run()
	if err != nil {
		slog.Error("selector.Run",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	if index == len(providers) {
		os.Exit(0)
	}

	p := providers[index]
	defaultModel := provider.Default(p.name())

	var model, description string
	switch p.name() {
	case "copilot":
		model, description = addCopilot(p.Prefix, defaultModel)

	case "compat":
		model, description = addCompat()

	case "codex":
		model, description = addOpenAICodex(p.Prefix, defaultModel)

	default:
		addAPIKey(p.label(), p.envKey())
		model, description = getModelName(p.Prefix, defaultModel)
	}

	if model != "" {
		upsertModel(model, description)
	}
}

func addCopilot(prefix, defaultModel string) (string, string) {
	_, err := copilot.New()
	if err != nil {
		slog.Error("failed to initialize Copilot",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	return getModelName(prefix, defaultModel)
}

func addCompat() (string, string) {
	nameInput := promptui.Prompt{
		Label: "Provider name (ex. ollama)",
		Validate: func(s string) error {
			s = strings.TrimSpace(s)
			if s == "" {
				return fmt.Errorf("provider name cannot be empty")
			}
			if strings.ContainsAny(s, " \t[]@") {
				return fmt.Errorf("name must not contain spaces, brackets or @")
			}
			return nil
		},
	}

	providor, err := nameInput.Run()
	if err != nil {
		slog.Error("nameInput.Run",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	providor = strings.ToUpper(strings.TrimSpace(providor))

	urlInput := promptui.Prompt{
		Label:   "URL (leave empty for http://localhost:11434)",
		Default: "",
	}
	url, err := urlInput.Run()
	if err != nil {
		slog.Error("urlInput.Run",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if url == "" {
		url = "http://localhost:11434"
	}

	if err := session.UpsertCompat(providor, url); err != nil {
		slog.Error("keychain.UpsertCompat",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	fmt.Printf("[*] Compat provider %q saved: %s\n", providor, url)

	fmt.Print("API Key (leave empty to skip): ")
	keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		slog.Error("term.ReadPassword", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		for i := range keyBytes {
			keyBytes[i] = 0
		}
	}()
	apiKey := strings.TrimSpace(string(keyBytes))
	if apiKey != "" {
		keychainKey := "COMPAT_" + providor + "_API_KEY"
		if err := keychain.Set(keychainKey, apiKey); err != nil {
			slog.Error("keychain.Set",
				slog.String("error", err.Error()))
			os.Exit(1)
		}
		fmt.Printf("[*] %s saved\n", keychainKey)
	} else {
		fmt.Printf("[*] No API key: %q\n", providor)
	}

	prefix := fmt.Sprintf("compat[%s]@", providor)
	model, _ := getModelName(prefix, "")
	return model, ""
}

func addAPIKey(label, envKey string) {
	if existing := keychain.Get(envKey); existing != "" {
		confirm := promptui.Select{
			Label:        fmt.Sprintf("%s API Key exist, replace with new?", label),
			Items:        []string{"No", "Yes"},
			HideSelected: true,
		}
		idx, _, err := confirm.Run()
		if err != nil {
			os.Exit(1)
		}
		if idx == 0 {
			return
		}
	}

	fmt.Printf("%s API Key: ", label)
	keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		slog.Error("term.ReadPassword", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		for i := range keyBytes {
			keyBytes[i] = 0
		}
	}()
	apiKey := strings.TrimSpace(string(keyBytes))
	if apiKey == "" {
		slog.Error("API key is required")
		os.Exit(1)
	}
	if err := keychain.Set(envKey, apiKey); err != nil {
		slog.Error("keychain.Set",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	fmt.Printf("[*] %s saved\n", envKey)
}

func getModelName(prefix, defaultModel string) (string, string) {
	p := strings.TrimSuffix(prefix, "@")
	if idx := strings.Index(p, "["); idx != -1 {
		p = ""
	}

	if p != "" {
		if model, desc, ok := selectModelFromList(prefix, p, defaultModel); ok {
			return model, desc
		}
	}

	modelInput := promptui.Prompt{
		Label:   fmt.Sprintf("Model name (prefix: %q)", prefix),
		Default: defaultModel,
	}
	model, err := modelInput.Run()
	if err != nil {
		os.Exit(1)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultModel
	}
	if model == "" {
		return "", ""
	}
	return prefix + model, ""
}

func selectModelFromList(prefix, providerName, defaultModel string) (model, desc string, ok bool) {
	models := provider.Models(providerName)
	if len(models) == 0 {
		return "", "", false
	}

	type entry struct {
		name string
		info provider.ModelItem
	}

	entries := make([]entry, 0, len(models))
	if info, exists := models[defaultModel]; exists {
		entries = append(entries, entry{defaultModel, info})
	}
	others := make([]string, 0, len(models))
	for name := range models {
		if name != defaultModel {
			others = append(others, name)
		}
	}
	sort.Strings(others)
	for _, name := range others {
		entries = append(entries, entry{name, models[name]})
	}

	items := make([]string, len(entries)+1)
	for i, e := range entries {
		items[i] = fmt.Sprintf("%-36s  %s", e.name, e.info.Description)
	}
	items[len(entries)] = "exit"

	selector := promptui.Select{
		Label:        fmt.Sprintf("Select %s", providerName),
		Items:        items,
		HideSelected: true,
		Size:         10,
	}
	idx, _, err := selector.Run()
	if err != nil {
		os.Exit(1)
	}

	if idx == len(entries) {
		os.Exit(0)
	}
	selected := entries[idx]
	return prefix + selected.name, selected.info.Description, true
}

func runReasoning() {
	cfg, err := session.Load()
	if err != nil {
		slog.Error("session.Load", slog.String("error", err.Error()))
		os.Exit(1)
	}

	current := cfg.ReasoningLevel
	if current == "" {
		current = "medium"
	}

	selector := promptui.Select{
		Label:        fmt.Sprintf("Select reasoning level (current: %s)", current),
		Items:        []string{"low", "medium", "high"},
		HideSelected: true,
	}
	_, level, err := selector.Run()
	if err != nil {
		os.Exit(1)
	}

	cfg.ReasoningLevel = level
	if err := session.Save(cfg); err != nil {
		slog.Error("session.Save", slog.String("error", err.Error()))
		os.Exit(1)
	}
	fmt.Printf("[*] reasoning level set to %q\n", level)
}

func runPlanner() {
	cfg, err := session.Load()
	if err != nil {
		slog.Error("session.Load",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	if len(cfg.Models) == 0 {
		fmt.Println("No models added. Run 'add' first.")
		return
	}

	items := make([]string, len(cfg.Models)+1)
	for i, m := range cfg.Models {
		items[i] = m.Name
	}
	items[len(cfg.Models)] = "exit"

	selector := promptui.Select{
		Label:        "Select planner model",
		Items:        items,
		HideSelected: true,
	}
	idx, _, err := selector.Run()
	if err != nil {
		os.Exit(1)
	}
	if idx == len(cfg.Models) {
		os.Exit(0)
	}

	cfg.PlannerModel = cfg.Models[idx].Name
	if err := session.Save(cfg); err != nil {
		slog.Error("session.Save",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	fmt.Printf("[*] set %q as planner model\n", cfg.PlannerModel)
}

func upsertModel(name, defaultDesc string) {
	descriptionInput := promptui.Prompt{
		Label:   "Model description",
		Default: defaultDesc,
	}

	description, err := descriptionInput.Run()
	if err != nil {
		os.Exit(1)
	}
	description = strings.TrimSpace(description)
	if description == "" {
		description = defaultDesc
	}

	cfg, err := session.Load()
	if err != nil {
		slog.Error("session.Load",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	seen := make(map[string]struct{})
	deduped := make([]session.ModelEntry, 0, len(cfg.Models))
	found := false
	for _, m := range cfg.Models {
		if _, ok := seen[m.Name]; ok {
			continue
		}
		seen[m.Name] = struct{}{}
		if m.Name == name {
			m.Description = description
			found = true
		}
		deduped = append(deduped, m)
	}
	cfg.Models = deduped
	if !found {
		cfg.Models = append(cfg.Models, session.ModelEntry{
			Name:        name,
			Description: description,
		})
	}

	if cfg.PlannerModel == "" {
		cfg.PlannerModel = name
	}

	if err := session.Save(cfg); err != nil {
		slog.Error("session.Save",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	fmt.Printf("[*] %q added\n", name)

	if cfg.PlannerModel == name {
		fmt.Printf("[*] set %q as planner model\n", name)
	}
}

func addOpenAICodex(prefix, defaultModel string) (string, string) {
	if openaicodex.HasToken() {
		confirm := promptui.Select{
			Label:        "OpenAI Codex token exists, re-login?",
			Items:        []string{"No", "Yes"},
			HideSelected: true,
		}
		idx, _, err := confirm.Run()
		if err != nil {
			os.Exit(1)
		}
		if idx == 0 {
			return getModelName(prefix, defaultModel)
		}
		if err := openaicodex.ClearToken(); err != nil {
			slog.Error("openaicodex.ClearToken", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	fmt.Println("[!] OpenAI Codex OAuth Notice")
	fmt.Println("[*] Authenticates via your personal ChatGPT account (OAuth), not an API key")
	fmt.Println("[*] Requires an active ChatGPT Pro or Max subscription")
	fmt.Println("[*] For personal testing only — not for commercial or multi-user use")
	fmt.Println()
	fmt.Print("[?] Press Enter to confirm you understand and accept the above. (Ctrl+C to cancel)")
	fmt.Scanln()

	_, err := openaicodex.New()
	if err != nil {
		slog.Error("failed to initialize OpenAI Codex",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	return getModelName(prefix, defaultModel)
}
