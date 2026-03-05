package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/utils"
)

type Token struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type Agent struct {
	httpClient *http.Client
	model      string
	Token      *Token
	Refresh    *RefreshToken
	workDir    string
	tokenDir   string
}

const (
	// gpt-4.1      1m/32k
	// gpt-4.1-mini 1m/32k
	// gpt-5-mini   400k/128k
	// gpt-4o       128k/4k
	defaultModel = "gpt-4.1"
	prefix       = "copilot@"
)

func New(model ...string) (*Agent, error) {
	usedModel := defaultModel
	if len(model) > 0 && strings.HasPrefix(model[0], prefix) {
		usedModel = strings.TrimPrefix(model[0], prefix)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd: %w", err)
	}

	configDir, err := utils.GetConfigDir()
	if err != nil {
		return nil, fmt.Errorf("utils.ConfigDir(: %w", err)
	}

	agent := &Agent{
		httpClient: &http.Client{},
		model:      usedModel,
		workDir:    workDir,
		tokenDir:   filepath.Join(configDir.Home, "copilot_token.json"),
	}

	var token *Token

	data, err := os.ReadFile(agent.tokenDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// * if is not exist, then login, github copilot code expire in 900s
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()

			token, err = agent.Login(ctx)
			if err != nil {
				return nil, fmt.Errorf("agent.Login: %w", err)
			}
			agent.Token = token
			return agent, nil
		}
		return nil, fmt.Errorf("os.ReadFile: %w", err)
	}

	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	agent.Token = token

	return agent, nil
}

func (a *Agent) Name() string {
	return a.model
}
