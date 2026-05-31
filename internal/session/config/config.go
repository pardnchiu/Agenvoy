package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

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
	AdminChannel     string        `json:"admin_channel"`
}

type ModelEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CompatEntry struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
}

func Load() (*Config, error) {
	if !go_pkg_filesystem_reader.Exists(filesystem.ConfigPath) {
		return &Config{}, nil
	}

	cfg, err := go_pkg_filesystem.ReadJSON[Config](filesystem.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadJSON [%s]: %w", filesystem.ConfigPath, err)
	}
	return &cfg, nil
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

func Get() (map[string]any, error) {
	dic, err := go_pkg_filesystem.ReadJSON[map[string]any](filesystem.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadJSON [%s]: %w", filesystem.ConfigPath, err)
	}
	if dic == nil {
		dic = map[string]any{}
	}
	return dic, nil
}

func Write(dic map[string]any) error {
	if err := go_pkg_filesystem.WriteJSON(filesystem.ConfigPath, dic, false); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteJSON [%s]: %w", filesystem.ConfigPath, err)
	}
	return nil
}

func Save(cfg *Config) error {
	oldDic, err := Get()
	if err != nil {
		oldDic = map[string]any{}
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("json Marshal: %w", err)
	}
	var newDic map[string]any
	if err := json.Unmarshal(raw, &newDic); err != nil {
		return fmt.Errorf("json Unmarshal %w", err)
	}

	maps.Copy(oldDic, newDic)
	delete(oldDic, "planner_model")
	return Write(oldDic)
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
