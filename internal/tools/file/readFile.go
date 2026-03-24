package file

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

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

			content, _, err := readFile(e, params.Path)
			if err != nil {
				return "", fmt.Errorf("file.readFile: %w", err)
			}
			return content, nil
		},
	})
}

func readFile(e *toolTypes.Executor, path string) (string, string, error) {
	absPath, err := filesystem.AbsPath(e.WorkDir, path, true)
	if err != nil {
		return "", "", fmt.Errorf("filesystem.AbsPath: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", absPath, fmt.Errorf("os.ReadFile: %w", err)
	}
	return string(data), absPath, nil
}
