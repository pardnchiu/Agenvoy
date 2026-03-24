package filesystem

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pardnchiu/agenvoy/configs"
)

var (
	deniedMapOnce sync.Once
	deniedMap     struct {
		Dirs       []string `json:"dirs"`
		Files      []string `json:"files"`
		Prefixes   []string `json:"prefixes"`
		Extensions []string `json:"extensions"`
	}
)

func isDenied(path string) bool {
	deniedMapOnce.Do(func() {
		if err := json.Unmarshal(configs.DeniedMap, &deniedMap); err != nil {
			slog.Warn("json.Unmarshal",
				slog.String("error", err.Error()))
		}
	})

	realPath, err := realPath(path)
	if err != nil {
		return true
	}

	cleanPath := filepath.Clean(realPath)
	basePath := filepath.Base(cleanPath)

	for _, dir := range deniedMap.Dirs {
		if strings.Contains(cleanPath, fmt.Sprintf("/%s/", dir)) ||
			strings.Contains(cleanPath, fmt.Sprintf("/%s", dir)) {
			return true
		}
	}

	for _, f := range deniedMap.Files {
		if strings.Contains(cleanPath, f) {
			return true
		}
	}

	// * skipped like .env.prod, but pass the .env.example
	for _, prefix := range deniedMap.Prefixes {
		if strings.HasPrefix(basePath, prefix) && !strings.Contains(basePath, ".example") {
			return true
		}
	}

	for _, ext := range deniedMap.Extensions {
		if strings.HasSuffix(basePath, ext) {
			return true
		}
	}
	return false
}
