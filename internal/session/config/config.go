package config

import (
	"encoding/json"
	"maps"
	"slices"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type Config struct {
	DispatcherModel  string            `json:"dispatcher_model"`
	SummaryModel     string            `json:"summary_model"`
	Models           []ModelEntry      `json:"models"`
	Compats          []CompatEntry     `json:"compats"`
	Keys             []string          `json:"keys"`
	DiscordEnabled   bool              `json:"discord_enabled"`
	DiscordUsername  string            `json:"discord_username"`
	TelegramEnabled  bool              `json:"telegram_enabled"`
	TelegramUsername string            `json:"telegram_username"`
	LineEnabled      bool              `json:"line_enabled"`
	LineUsername     string            `json:"line_username"`
	ImageGenerator   string            `json:"image_generator"`
	DecisionModel    string            `json:"decision_model"`
	STTModel         string            `json:"stt_model"`
	TTSModel         string            `json:"tts_model"`
	AdminChannel     string            `json:"admin_channel"`
	OutputDir        string            `json:"output_dir"`
	OfficialGuideOff bool              `json:"official_guide_disabled"`
	SkillSourceOff   []string          `json:"skill_source_disabled"`
	ModelTag         map[string]string `json:"model_tag"`
}

const TypesafeKey = "TYPESAFE_API_KEY"

type ModelEntry struct {
	Name string
}

func (m ModelEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Name)
}

func (m *ModelEntry) UnmarshalJSON(raw []byte) error {
	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		m.Name = name
		return nil
	}
	var legacy struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	m.Name = legacy.Name
	return nil
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
		return nil, utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.ReadJSON", filesystem.ConfigPath, err)
	}
	normalizeModels(&cfg)
	return &cfg, nil
}

func Get() (map[string]any, error) {
	dic, err := go_pkg_filesystem.ReadJSON[map[string]any](filesystem.ConfigPath)
	if err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.ReadJSON", filesystem.ConfigPath, err)
	}
	if dic == nil {
		dic = map[string]any{}
	}
	return dic, nil
}

func Write(dic map[string]any) error {
	if err := go_pkg_filesystem.WriteJSON(filesystem.ConfigPath, dic, false); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteJSON", filesystem.ConfigPath, err)
	}
	return nil
}

func Save(cfg *Config) error {
	normalizeModels(cfg)

	oldDic, err := Get()
	if err != nil {
		oldDic = map[string]any{}
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "config", err)
	}
	var newDic map[string]any
	if err := json.Unmarshal(raw, &newDic); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "json.Unmarshal", "config", err)
	}

	maps.Copy(oldDic, newDic)
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

func DeleteKey(key string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	if idx := slices.Index(cfg.Keys, key); idx != -1 {
		cfg.Keys = slices.Delete(cfg.Keys, idx, idx+1)
		return Save(cfg)
	}
	return nil
}
