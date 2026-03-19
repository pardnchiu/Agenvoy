package compat

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/keychain"
)

type Agent struct {
	httpClient *http.Client
	model      string
	baseURL    string
	apiKey     string
	workDir    string
}

const (
	defaultModel   = "qwen3:8b"
	defaultBaseURL = "http://localhost:11434"
)

func New(model ...string) (*Agent, error) {
	usedModel := defaultModel
	instanceName := ""

	if len(model) > 0 && model[0] != "" {
		raw := model[0]
		if start := strings.Index(raw, "["); start != -1 {
			if end := strings.Index(raw, "]"); end > start {
				instanceName = strings.ToUpper(raw[start+1 : end])
			}
		}
		if at := strings.Index(raw, "@"); at != -1 {
			usedModel = raw[at+1:]
		}
	}

	urlEnvKey := "COMPAT_URL"
	apiKeyEnvKey := "COMPAT_API_KEY"
	if instanceName != "" {
		urlEnvKey = "COMPAT_" + instanceName + "_URL"
		apiKeyEnvKey = "COMPAT_" + instanceName + "_API_KEY"
	}

	baseURL := keychain.Get(urlEnvKey)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	apiKey := keychain.Get(apiKeyEnvKey)

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd: %w", err)
	}

	return &Agent{
		httpClient: &http.Client{Timeout: 2 * time.Minute},
		model:      usedModel,
		baseURL:    baseURL,
		apiKey:     apiKey,
		workDir:    workDir,
	}, nil
}

func (a *Agent) Name() string {
	return a.model
}

func (a *Agent) MaxInputTokens() int {
	return 128000
}
