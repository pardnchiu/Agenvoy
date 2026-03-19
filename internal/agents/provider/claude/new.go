package claude

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/agenvoy/internal/filesystem/keychain"
)

type Agent struct {
	httpClient *http.Client
	model      string
	apiKey     string
	workDir    string
}

const (
	prefix = "claude@"
)

func New(model ...string) (*Agent, error) {
	usedModel := provider.Default("claude")
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
		httpClient: &http.Client{Timeout: 2 * time.Minute},
		model:      usedModel,
		apiKey:     apiKey,
		workDir:    workDir,
	}, nil
}

func (a *Agent) Name() string {
	return a.model
}

func (a *Agent) MaxInputTokens() int {
	return provider.Get("claude", a.model).Input
}
