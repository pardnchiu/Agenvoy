package claude

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/provider"
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
	if len(model) == 0 || !strings.HasPrefix(model[0], prefix) {
		return nil, fmt.Errorf("claude.New: model arg required with %q prefix", prefix)
	}
	usedModel := strings.TrimPrefix(model[0], prefix)

	apiKey := keychain.Get("CLAUDE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("keychain.Get: CLAUDE_API_KEY is required")
	}

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd: %w", err)
	}

	return &Agent{
		httpClient: provider.NewHTTPClient(),
		model:      usedModel,
		apiKey:     apiKey,
		workDir:    workDir,
	}, nil
}

func (a *Agent) Name() string {
	return a.model
}

func (a *Agent) maxOutputTokens() int {
	switch {
	case strings.HasPrefix(a.model, "claude-opus-4-6"),
		strings.HasPrefix(a.model, "claude-opus-4-7"):
		return 128000
	case strings.HasPrefix(a.model, "claude-opus-4-1-"),
		strings.HasPrefix(a.model, "claude-opus-4-2"):
		return 32000
	}
	return 64000
}
