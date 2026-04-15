package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/go-utils/filesystem/keychain"
)

type Token struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope"`
	ExpiresAt   time.Time `json:"expires_at"`
}

const tokenKey = "agenvoy.copilot.token"

type Agent struct {
	httpClient *http.Client
	model      string
	Token      *Token
	Refresh    *RefreshToken
	workDir    string
}

const (
	prefix = "copilot@"
)

func New(model ...string) (*Agent, error) {
	usedModel := provider.Default("copilot")
	if len(model) > 0 && strings.HasPrefix(model[0], prefix) {
		usedModel = strings.TrimPrefix(model[0], prefix)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd: %w", err)
	}

	// configDir, err := utils.GetConfigDir()
	// if err != nil {
	// 	return nil, fmt.Errorf("utils.ConfigDir(: %w", err)
	// }

	agent := &Agent{
		httpClient: &http.Client{Timeout: 2 * time.Minute},
		model:      usedModel,
		workDir:    workDir,
	}

	raw := keychain.Get(tokenKey)
	if raw == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		token, err := agent.Login(ctx)
		if err != nil {
			return nil, fmt.Errorf("agent.Login: %w", err)
		}
		agent.Token = token
		return agent, nil
	}

	var token Token
	if err := json.Unmarshal([]byte(raw), &token); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	agent.Token = &token

	return agent, nil
}

func (a *Agent) Name() string {
	return a.model
}
