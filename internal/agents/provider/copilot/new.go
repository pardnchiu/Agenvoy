package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/provider"
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
	if len(model) == 0 || !strings.HasPrefix(model[0], prefix) {
		return nil, fmt.Errorf("copilot.New: model arg required with %q prefix", prefix)
	}
	usedModel := strings.TrimPrefix(model[0], prefix)

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd: %w", err)
	}

	agent := &Agent{
		httpClient: provider.NewHTTPClient(),
		model:      usedModel,
		workDir:    workDir,
	}

	raw := keychain.Get(tokenKey)
	if raw == "" {
		return nil, fmt.Errorf("copilot token missing; run `agen model add` to authenticate")
	}

	var token Token
	if err := json.Unmarshal([]byte(raw), &token); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	agent.Token = &token

	return agent, nil
}

func (a *Agent) Name() string {
	return prefix + a.model
}

func HasToken() bool {
	return keychain.Get(tokenKey) != ""
}

func ClearToken() error {
	return keychain.Delete(tokenKey)
}

func AuthWithCallback(ctx context.Context, onCode func(*DeviceCode)) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("os.Getwd: %w", err)
	}
	a := &Agent{
		httpClient: provider.NewHTTPClient(),
		workDir:    workDir,
	}
	token, err := a.LoginWithCallback(ctx, onCode)
	if err != nil {
		return fmt.Errorf("a.LoginWithCallback: %w", err)
	}
	a.Token = token
	return nil
}
