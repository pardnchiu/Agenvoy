package openaicodex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

const prefix = "codex@"

func newHTTPClient() *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	transport.ResponseHeaderTimeout = 10 * time.Second
	return &http.Client{
		Timeout:   10 * time.Minute,
		Transport: transport,
	}
}

type Agent struct {
	httpClient *http.Client
	model      string
	workDir    string

	token *StoredToken
}

func New(model ...string) (*Agent, error) {
	if len(model) == 0 || !strings.HasPrefix(model[0], prefix) {
		return nil, fmt.Errorf("openaicodex.New: model arg required with %q prefix", prefix)
	}
	usedModel := strings.TrimPrefix(model[0], prefix)

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd: %w", err)
	}

	a := &Agent{
		httpClient: newHTTPClient(),
		model:      usedModel,
		workDir:    workDir,
	}

	raw := keychain.Get(tokenKey)
	if raw == "" {
		return nil, fmt.Errorf("codex token missing; run `agen model add` to authenticate")
	}

	var stored StoredToken
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	a.token = &stored

	if stored.expired() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.refreshToken(ctx); err != nil {
			return nil, fmt.Errorf("codex token expired and refresh failed: %w; run `agen model add` to re-authenticate", err)
		}
	}

	return a, nil
}

func Authenticate(ctx context.Context) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("os.Getwd: %w", err)
	}
	a := &Agent{
		httpClient: newHTTPClient(),
		workDir:    workDir,
	}
	token, err := a.Login(ctx)
	if err != nil {
		return fmt.Errorf("a.Login: %w", err)
	}
	a.token = token
	return nil
}

func (a *Agent) Name() string {
	return prefix + a.model
}

func (a *Agent) authHeader(ctx context.Context) (string, error) {
	if err := a.ensureFreshToken(ctx); err != nil {
		return "", fmt.Errorf("a.ensureFreshToken: %w", err)
	}
	return "Bearer " + a.token.AccessToken, nil
}
