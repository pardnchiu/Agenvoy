package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

const registryEmailKey = "registry_email"

func configPath() string {
	return filepath.Join(filesystem.AgenvoyDir, "config.json")
}

func readConfigMap() (map[string]any, error) {
	path := configPath()
	if !go_pkg_filesystem_reader.Exists(path) {
		return map[string]any{}, nil
	}
	m, err := go_pkg_filesystem.ReadJSON[map[string]any](path)
	if err != nil {
		return nil, fmt.Errorf("go_pkg_filesystem.ReadJSON: %w", err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func writeConfigMap(m map[string]any) error {
	path := configPath()
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
	}
	if err := go_pkg_filesystem.WriteJSON(path, m, false); err != nil {
		return fmt.Errorf("go_pkg_filesystem.WriteJSON: %w", err)
	}
	return nil
}

func registRegistryEmail() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "get_registry_email",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Timeout:     10 * time.Second,
		Description: "Read the marketplace registry email from ~/.config/agenvoy/config.json. Returns {email: \"\"} when not set. Use before publishing extensions (extension-upload skill) to decide if first-time email setup is needed.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			m, err := readConfigMap()
			if err != nil {
				return "", err
			}
			email, _ := m[registryEmailKey].(string)
			out, err := json.Marshal(map[string]any{"email": email})
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(out), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "set_registry_email",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Timeout:     10 * time.Second,
		Description: "Save the marketplace registry email to ~/.config/agenvoy/config.json. Validates strict email format ^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"email": map[string]any{
					"type":        "string",
					"description": "Email address. Must match ^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$.",
				},
			},
			"required": []string{"email"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var p struct {
				Email string `json:"email"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			email := strings.TrimSpace(p.Email)
			if email == "" {
				return "", fmt.Errorf("email is required")
			}
			if !emailPattern.MatchString(email) {
				return "", fmt.Errorf("invalid email format: %q", email)
			}

			m, err := readConfigMap()
			if err != nil {
				return "", err
			}
			m[registryEmailKey] = email
			if err := writeConfigMap(m); err != nil {
				return "", err
			}

			out, err := json.Marshal(map[string]any{"ok": true, "email": email})
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(out), nil
		},
	})
}
