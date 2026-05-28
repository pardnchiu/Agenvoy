package session

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var providerStaticKeys = []string{
	"CLAUDE_API_KEY",
	"OPENAI_API_KEY",
	"GEMINI_API_KEY",
	"GROK_API_KEY",
	"DEEPSEEK_API_KEY",
	"NVIDIA_API_KEY",
}

type CompatEntry struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
}

type ModelEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Config struct {
	DispatcherModel  string        `json:"dispatcher_model"`
	SummaryModel     string        `json:"summary_model"`
	ReasoningLevel   string        `json:"reasoning_level"`
	Models           []ModelEntry  `json:"models"`
	Compats          []CompatEntry `json:"compats"`
	Keys             []string      `json:"keys"`
	DiscordGuildID   string        `json:"discord_guild_id"`
	DiscordEnabled   bool          `json:"discord_enabled"`
	DiscordUsername  string        `json:"discord_username"`
	TelegramEnabled  bool          `json:"telegram_enabled"`
	TelegramUsername string        `json:"telegram_username"`
	KuradbEnabled    bool          `json:"kuradb_enabled"`
}

func (c *Config) UnmarshalJSON(data []byte) error {
	type alias Config
	aux := struct {
		*alias
		LegacyPlannerModel string `json:"planner_model,omitempty"`
	}{alias: (*alias)(c)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if c.DispatcherModel == "" && aux.LegacyPlannerModel != "" {
		c.DispatcherModel = aux.LegacyPlannerModel
	}
	return nil
}

func Load() (*Config, error) {
	configPath := filepath.Join(filesystem.AgenvoyDir, "config.json")
	if !go_pkg_filesystem_reader.Exists(configPath) {
		return &Config{}, nil
	}
	cfg, err := go_pkg_filesystem.ReadJSON[Config](configPath)
	if err != nil {
		return nil, fmt.Errorf("go_pkg_filesystem.ReadJSON: %w", err)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	configPath := filepath.Join(filesystem.AgenvoyDir, "config.json")
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(configPath), true); err != nil {
		return fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
	}

	existing := map[string]any{}
	if go_pkg_filesystem_reader.Exists(configPath) {
		if m, err := go_pkg_filesystem.ReadJSON[map[string]any](configPath); err == nil && m != nil {
			existing = m
		}
	}

	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("json.Marshal cfg: %w", err)
	}
	var cfgMap map[string]any
	if err := json.Unmarshal(cfgBytes, &cfgMap); err != nil {
		return fmt.Errorf("json.Unmarshal cfg: %w", err)
	}
	for k, v := range cfgMap {
		existing[k] = v
	}
	delete(existing, "planner_model")

	return go_pkg_filesystem.WriteJSON(configPath, existing, false)
}

func UpsertCompat(provider, url string) error {
	provider = strings.ToUpper(strings.TrimSpace(provider))
	cfg, err := Load()
	if err != nil {
		return err
	}

	for i, c := range cfg.Compats {
		if strings.EqualFold(c.Provider, provider) {
			cfg.Compats[i].URL = url
			return Save(cfg)
		}
	}
	cfg.Compats = append(cfg.Compats, CompatEntry{
		Provider: provider,
		URL:      url,
	})
	return Save(cfg)
}

func SaveKey(key string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	if slices.Contains(cfg.Keys, key) {
		return nil
	}
	cfg.Keys = append(cfg.Keys, key)
	return Save(cfg)
}

func BackfillKeys() error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	candidates := append([]string{}, providerStaticKeys...)
	for _, c := range cfg.Compats {
		candidates = append(candidates, "COMPAT_"+strings.ToUpper(c.Provider)+"_API_KEY")
	}

	changed := false
	for _, k := range candidates {
		if slices.Contains(cfg.Keys, k) {
			continue
		}
		if keychain.Get(k) == "" {
			continue
		}
		cfg.Keys = append(cfg.Keys, k)
		changed = true
	}
	if !changed {
		return nil
	}
	return Save(cfg)
}

func IsKeyExist(key string) bool {
	cfg, err := Load()
	if err != nil {
		return false
	}
	return slices.Contains(cfg.Keys, key)
}

func GetCompatURL(provider string) string {
	cfg, err := Load()
	if err != nil {
		return ""
	}

	for _, c := range cfg.Compats {
		if strings.EqualFold(c.Provider, provider) {
			return c.URL
		}
	}
	return ""
}
