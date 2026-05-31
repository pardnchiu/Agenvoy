package config

import (
	"slices"
	"strings"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

var providerStaticKeys = []string{
	"CLAUDE_API_KEY",
	"OPENAI_API_KEY",
	"GEMINI_API_KEY",
	"GROK_API_KEY",
	"DEEPSEEK_API_KEY",
	"NVIDIA_API_KEY",
}

func UpsertCompat(provider, url string) error {
	provider = strings.ToUpper(strings.TrimSpace(provider))
	cfg, err := Load()
	if err != nil {
		return err
	}

	for k, v := range cfg.Compats {
		if strings.EqualFold(v.Provider, provider) {
			cfg.Compats[k].URL = url
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

	for _, v := range cfg.Compats {
		if strings.EqualFold(v.Provider, provider) {
			return v.URL
		}
	}
	return ""
}

// * auto fulfill keys on config before v0.24.15
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
