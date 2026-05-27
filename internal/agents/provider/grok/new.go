package grok

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
	prefix = "grok@"
)

func New(model ...string) (*Agent, error) {
	if len(model) == 0 || !strings.HasPrefix(model[0], prefix) {
		return nil, fmt.Errorf("grok.New: model arg required with %q prefix", prefix)
	}
	usedModel := strings.TrimPrefix(model[0], prefix)

	apiKey := keychain.Get("GROK_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("keychain.Get: GROK_API_KEY is required")
	}

	workDir, _ := os.Getwd()

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
