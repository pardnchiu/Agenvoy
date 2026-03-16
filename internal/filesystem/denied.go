package filesystem

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
)

type deniedConfig struct {
	Dirs       []string `json:"dirs"`
	Files      []string `json:"files"`
	Prefixes   []string `json:"prefixes"`
	Extensions []string `json:"extensions"`
}

var DeniedConfig = func() deniedConfig {
	var cfg deniedConfig
	if err := json.Unmarshal(configs.DeniedMap, &cfg); err != nil {
		slog.Warn("json.Unmarshal",
			slog.String("error", err.Error()))
	}
	return cfg
}()

func isDenied(path string) bool {
	cleaned := filepath.Clean(path)
	base := filepath.Base(cleaned)

	for _, dir := range DeniedConfig.Dirs {
		if strings.Contains(cleaned, fmt.Sprintf("/%s/", dir)) || strings.Contains(cleaned, fmt.Sprintf("/%s", dir)) {
			return true
		}
	}
	for _, f := range DeniedConfig.Files {
		if strings.Contains(cleaned, f) {
			return true
		}
	}
	for _, prefix := range DeniedConfig.Prefixes {
		if strings.HasPrefix(base, prefix) && !strings.Contains(base, ".example") {
			return true
		}
	}
	for _, ext := range DeniedConfig.Extensions {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	return false
}
