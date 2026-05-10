package nvidia

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

type Agent struct {
	httpClient *http.Client
	model      string
	apiKey     string
	workDir    string
}

const (
	prefix = "nvidia@"
)

func New(model ...string) (*Agent, error) {
	if len(model) == 0 || !strings.HasPrefix(model[0], prefix) {
		return nil, fmt.Errorf("nvidia.New: model arg required with %q prefix", prefix)
	}
	usedModel := strings.TrimPrefix(model[0], prefix)

	apiKey := keychain.Get("NVIDIA_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("keychain.Get: NVIDIA_API_KEY is required")
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
