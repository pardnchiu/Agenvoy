package openai

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
	prefix = "openai@"
)

func New(model ...string) (*Agent, error) {
	usedModel := provider.Default("openai")
	if len(model) > 0 && strings.HasPrefix(model[0], prefix) {
		usedModel = strings.TrimPrefix(model[0], prefix)
	}
	apiKey := keychain.Get("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("keychain.Get: OPENAI_API_KEY is required")
	}

	workDir, _ := os.Getwd()

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
