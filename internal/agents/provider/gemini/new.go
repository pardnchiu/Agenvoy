package gemini

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
	// gemini-2.5-pro   1m/64k
	// gemini-2.5-flash 1m/64k
	defaultModel = "gemini-2.5-pro"
	prefix       = "gemini@"
)

func New(model ...string) (*Agent, error) {
	usedModel := defaultModel
	if len(model) > 0 && strings.HasPrefix(model[0], prefix) {
		usedModel = strings.TrimPrefix(model[0], prefix)
	}

	apiKey := keychain.Get("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("keychain.Get: GEMINI_API_KEY is required")
	}

	workDir, _ := os.Getwd()

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
