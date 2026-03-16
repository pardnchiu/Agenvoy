package file

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
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

func registReadFile() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "read_file",
		Description: "讀取指定路徑的檔案內容。用於檢查原始碼、設定檔或專案中的任何文字檔案。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要讀取的檔案路徑（相對於專案根目錄或絕對路徑）",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			// TODO: remove this after remove isExclude
			absPath, err := filesystem.GetAbsPath(e.WorkPath, params.Path)
			if err != nil {
				return "", fmt.Errorf("filesystem.GetAbsPath: %w", err)
			}

			// TODO: need to move to filesystem
			if isExclude(e, absPath) {
				return "", fmt.Errorf("isExclude: %s", params.Path)
			}

			data, err := filesystem.ReadFile(e.WorkPath, params.Path)
			if err != nil {
				return "", fmt.Errorf("filesystem.ReadFile: %w", err)
			}
			return data, nil
		},
	})
}

func getFullPath(e *toolTypes.Executor, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return filepath.Join(e.WorkPath, path), nil
	}
	cleaned := filepath.Clean(path)
	homeDir, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(cleaned, filepath.Clean(homeDir)+string(filepath.Separator)) {
		return "", fmt.Errorf("only allow user home: %s", path)
	}
	return cleaned, nil
}

func isExclude(e *toolTypes.Executor, path string) bool {
	excluded := false
	for _, e := range e.Exclude {
		match, err := filepath.Match(e.File, filepath.Base(path))
		if err != nil {
			continue
		}

		if !match {
			match = strings.Contains(path, "/"+e.File+"/") ||
				strings.HasPrefix(path, e.File+"/")
		}
		if match {
			excluded = !e.Negate
		}
	}
	return excluded
}
