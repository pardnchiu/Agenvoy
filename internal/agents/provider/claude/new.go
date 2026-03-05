package claude

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/keychain"
)

type Agent struct {
	httpClient *http.Client
	model      string
	apiKey     string
	workDir    string
}

const (
	// claude-sonnet-4-5 200K/64000
	// claude-opus-4-6   200K/128K
	// claude-opus-4-5   200K/128K
	defaultModel = "claude-sonnet-4-5"
	prefix       = "claude@"
)

func New(model ...string) (*Agent, error) {
	usedModel := defaultModel
	if len(model) > 0 && strings.HasPrefix(model[0], prefix) {
		usedModel = strings.TrimPrefix(model[0], prefix)
	}
	apiKey := keychain.Get("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("keychain.Get: ANTHROPIC_API_KEY is required")
	}

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd: %w", err)
	}

	return &Agent{
		httpClient: &http.Client{},
		model:      usedModel,
		apiKey:     apiKey,
		workDir:    workDir,
	}, nil
}

func (a *Agent) Name() string {
	return a.model
}
