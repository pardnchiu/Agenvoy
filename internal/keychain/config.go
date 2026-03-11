package keychain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/utils"
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
	SessionID    string        `json:"session_id,omitempty"`
	PlannerModel string        `json:"planner_model,omitempty"`
	Models       []ModelEntry  `json:"models,omitempty"`
	Compats      []CompatEntry `json:"compats,omitempty"`
}

func Load() (*Config, error) {
	configData, err := utils.GetConfigDir()
	if err != nil {
		return nil, fmt.Errorf("utils.GetConfigDir: %w", err)
	}

	configPath := filepath.Join(configData.Home, "config.json")
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("os.ReadFile: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	configData, err := utils.GetConfigDir()
	if err != nil {
		return fmt.Errorf("utils.GetConfigDir: %w", err)
	}

	configPath := filepath.Join(configData.Home, "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	return utils.WriteFile(configPath, string(data), 0644)
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
