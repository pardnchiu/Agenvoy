package session

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type CompatEntry struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
}

type ModelEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Config struct {
	DispatcherModel  string        `json:"dispatcher_model,omitempty"`
	ReasoningLevel   string        `json:"reasoning_level,omitempty"`
	Models           []ModelEntry  `json:"models,omitempty"`
	Compats          []CompatEntry `json:"compats,omitempty"`
	Keys             []string      `json:"keys,omitempty"`
	DiscordGuildID   string        `json:"discord_guild_id,omitempty"`
	DiscordEnabled   bool          `json:"discord_enabled,omitempty"`
	DiscordUsername  string        `json:"discord_username,omitempty"`
	TelegramEnabled  bool          `json:"telegram_enabled,omitempty"`
	TelegramUsername string        `json:"telegram_username,omitempty"`
	KuradbEnabled    bool          `json:"kuradb_enabled,omitempty"`
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
	return go_pkg_filesystem.WriteJSON(configPath, cfg, false)
}

func UpsertModel(entry ModelEntry) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	for i, m := range cfg.Models {
		if m.Name == entry.Name {
			cfg.Models[i].Description = entry.Description
			return Save(cfg)
		}
	}
	cfg.Models = append(cfg.Models, entry)
	return Save(cfg)
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
