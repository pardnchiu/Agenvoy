package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/copilot"
	openaicodex "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
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

	var model, description string
	switch p.name() {
	case "copilot":
		model, description = addCopilot(p.Prefix)

	case "compat":
		model, description = addCompat()

	case "codex":
		model, description = addOpenAICodex(p.Prefix)

	default:
		addAPIKey(p.label(), p.envKey())
		model, description = getModelName(p.Prefix)
	}

	if model != "" {
		upsertModel(model, description)
	}
}

func addCopilot(prefix string) (string, string) {
	if copilot.HasToken() {
		confirm := promptui.Select{
			Label:        "Copilot token exists, re-login?",
			Items:        []string{"No", "Yes"},
			HideSelected: true,
		}
		idx, _, err := confirm.Run()
		if err != nil {
			os.Exit(1)
		}
		if idx == 0 {
			return getModelName(prefix)
		}
		if err := copilot.ClearToken(); err != nil {
			slog.Error("copilot.ClearToken", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if err := copilot.Authenticate(ctx); err != nil {
		slog.Error("failed to initialize Copilot",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	return getModelName(prefix)
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
		Label:   "URL (leave empty for http://localhost:11434/v1)",
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
		url = "http://localhost:11434/v1"
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
	model, _ := getModelName(prefix)
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

func getModelName(prefix string) (string, string) {
	p := strings.TrimSuffix(prefix, "@")
	if idx := strings.Index(p, "["); idx != -1 {
		p = ""
	}

	if p != "" {
		if model, desc, ok := selectModelFromList(prefix, p); ok {
			return model, desc
		}
	}

	modelInput := promptui.Prompt{
		Label: fmt.Sprintf("Model name (prefix: %q)", prefix),
	}
	model, err := modelInput.Run()
	if err != nil {
		os.Exit(1)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return "", ""
	}
	return prefix + model, ""
}

func selectModelFromList(prefix, providerName string) (model, desc string, ok bool) {
	models := provider.Models(providerName)
	if len(models) == 0 {
		return "", "", false
	}

	type entry struct {
		name string
		info provider.ModelItem
	}

	names := make([]string, 0, len(models))
	for name := range models {
		names = append(names, name)
	}
	sort.SliceStable(names, func(i, j int) bool {
		pi, pj := isPreviewModel(names[i]), isPreviewModel(names[j])
		if pi != pj {
			return !pi
		}
		return names[i] < names[j]
	})

	entries := make([]entry, 0, len(names))
	for _, name := range names {
		entries = append(entries, entry{name, models[name]})
	}

	const nameCol = 36
	const fixedOverhead = nameCol + 2 + 4
	cols, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || cols <= 0 {
		cols = 100
	}
	descBudget := max(cols-fixedOverhead, 20)

	items := make([]string, len(entries)+1)
	for i, e := range entries {
		items[i] = fmt.Sprintf("%-*s  %s", nameCol, e.name, truncateByWidth(e.info.Description, descBudget))
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

func runDispatcher() {
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
		Label:        "Select dispatcher model",
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

	cfg.DispatcherModel = cfg.Models[idx].Name
	if err := session.Save(cfg); err != nil {
		slog.Error("session.Save",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	fmt.Printf("[*] set %q as dispatcher model\n", cfg.DispatcherModel)
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

	if cfg.DispatcherModel == "" {
		cfg.DispatcherModel = name
	}

	if err := session.Save(cfg); err != nil {
		slog.Error("session.Save",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	fmt.Printf("[*] %q added\n", name)

	if cfg.DispatcherModel == name {
		fmt.Printf("[*] set %q as dispatcher model\n", name)
	}
}

func addOpenAICodex(prefix string) (string, string) {
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
			return getModelName(prefix)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := openaicodex.Authenticate(ctx); err != nil {
		slog.Error("failed to initialize OpenAI Codex",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	return getModelName(prefix)
}

func isPreviewModel(name string) bool {
	return strings.Contains(name, "-preview") || strings.Contains(name, "-experimental")
}

type brokenModel struct {
	Name     string
	Provider string
	Reason   string
}

func checkBrokenModels() []brokenModel {
	cfg, err := session.Load()
	if err != nil {
		return nil
	}
	out := make([]brokenModel, 0)
	for _, m := range cfg.Models {
		head := strings.SplitN(m.Name, "@", 2)[0]
		prov, _, _ := strings.Cut(head, "[")
		var reason string
		switch prov {
		case "copilot":
			if !copilot.HasToken() {
				reason = "copilot token missing"
			}
		case "codex":
			if !openaicodex.HasToken() {
				reason = "codex token missing"
			}
		case "openai", "claude", "gemini", "nvidia":
			envKey := strings.ToUpper(prov) + "_API_KEY"
			if keychain.Get(envKey) == "" {
				reason = envKey + " missing"
			}
		}
		if reason != "" {
			out = append(out, brokenModel{Name: m.Name, Provider: prov, Reason: reason})
		}
	}
	return out
}

func removeModel(name string) error {
	cfg, err := session.Load()
	if err != nil {
		return fmt.Errorf("session.Load: %w", err)
	}
	out := make([]session.ModelEntry, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		if m.Name != name {
			out = append(out, m)
		}
	}
	cfg.Models = out
	if cfg.DispatcherModel == name {
		cfg.DispatcherModel = ""
		if len(out) > 0 {
			cfg.DispatcherModel = out[0].Name
		}
	}
	if err := session.Save(cfg); err != nil {
		return fmt.Errorf("session.Save: %w", err)
	}
	return nil
}

func checkModels() {
	broken := checkBrokenModels()
	if len(broken) == 0 {
		return
	}

	fmt.Printf("[!] %d model(s) with auth issue:\n", len(broken))
	for _, b := range broken {
		fmt.Printf("    %s — %s\n", b.Name, b.Reason)
	}
	fmt.Println()

	for _, b := range broken {
		action := promptui.Select{
			Label:        fmt.Sprintf("%s — %s · action?", b.Name, b.Reason),
			Items:        []string{"Re-authenticate", "Remove from config"},
			HideSelected: true,
		}
		idx, _, err := action.Run()
		if err != nil {
			os.Exit(1)
		}
		if idx == 1 {
			if err := removeModel(b.Name); err != nil {
				slog.Error("removeModel",
					slog.String("name", b.Name),
					slog.String("error", err.Error()))
				os.Exit(1)
			}
			fmt.Printf("[*] %s removed\n", b.Name)
			continue
		}

		if err := reLogin(b); err != nil {
			fmt.Printf("[!] %s re-auth failed: %v\n", b.Name, err)
			os.Exit(1)
		}
		fmt.Printf("[*] %s re-authenticated\n", b.Name)
	}
}

func reLogin(b brokenModel) error {
	switch b.Provider {
	case "copilot":
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		return copilot.Authenticate(ctx)
	case "codex":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		return openaicodex.Authenticate(ctx)
	case "openai", "claude", "gemini", "nvidia":
		envKey := strings.ToUpper(b.Provider) + "_API_KEY"
		fmt.Printf("%s API Key: ", strings.ToUpper(b.Provider[:1])+b.Provider[1:])
		keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("term.ReadPassword: %w", err)
		}
		defer func() {
			for i := range keyBytes {
				keyBytes[i] = 0
			}
		}()
		apiKey := strings.TrimSpace(string(keyBytes))
		if apiKey == "" {
			return fmt.Errorf("api key is required")
		}
		if err := keychain.Set(envKey, apiKey); err != nil {
			return fmt.Errorf("keychain.Set: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown provider: %s", b.Provider)
	}
}

func truncateByWidth(s string, budget int) string {
	if budget <= 0 {
		return ""
	}
	used := 0
	for i, r := range s {
		rw := runeColumns(r)
		if used+rw > budget-1 {
			return s[:i] + "…"
		}
		used += rw
	}
	return s
}

func runeColumns(r rune) int {
	if r < 0x80 {
		if r < 0x20 || r == 0x7F {
			return 0
		}
		return 1
	}
	switch {
	case r >= 0x1100 && r <= 0x115F,
		r >= 0x2E80 && r <= 0x303E,
		r >= 0x3041 && r <= 0x33FF,
		r >= 0x3400 && r <= 0x4DBF,
		r >= 0x4E00 && r <= 0x9FFF,
		r >= 0xA000 && r <= 0xA4CF,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE30 && r <= 0xFE4F,
		r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x20000 && r <= 0x2FFFD,
		r >= 0x30000 && r <= 0x3FFFD:
		return 2
	}
	return 1
}
