package userData

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

var (
	emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

func registSetUserEmail() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "set_user_email",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Timeout:     10 * time.Second,
		Description: "Save the user email to config. Validates strict email format ^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$.",
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
				return "", fmt.Errorf("json Unmarshal: %w", err)
			}

			email := strings.TrimSpace(p.Email)
			if email == "" {
				return "", fmt.Errorf("email is required")
			}
			if !emailRegex.MatchString(email) {
				return "", fmt.Errorf("invalid format: %q", email)
			}

			dic, err := filesystem.ReadConfig()
			if err != nil {
				return "", err
			}
			dic["email"] = email

			if err := filesystem.WriteConfig(dic); err != nil {
				return "", err
			}

			bytes, err := json.Marshal(map[string]any{"ok": true, "email": email})
			if err != nil {
				return "", fmt.Errorf("json Marshal: %w", err)
			}
			return string(bytes), nil
		},
	})

}

func registGetUserEmail() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "get_user_email",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Timeout:     10 * time.Second,
		Description: "Read the user email from config. Returns {email: \"\"} when not set. Use before publishing extensions (extension-upload skill) to decide if first-time email setup is needed.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			dic, err := filesystem.ReadConfig()
			if err != nil {
				return "", err
			}

			email, _ := dic["email"].(string)
			bytes, err := json.Marshal(map[string]any{"email": email})
			if err != nil {
				return "", fmt.Errorf("json Marshal: %w", err)
			}
			return string(bytes), nil
		},
	})
}
