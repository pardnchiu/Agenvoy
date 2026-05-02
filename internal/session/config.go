package session

import (
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
	SessionID      string        `json:"session_id,omitempty"`
	PlannerModel   string        `json:"planner_model,omitempty"`
	ReasoningLevel string        `json:"reasoning_level,omitempty"`
	Models         []ModelEntry  `json:"models,omitempty"`
	Compats        []CompatEntry `json:"compats,omitempty"`
	Keys           []string      `json:"keys,omitempty"`
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
