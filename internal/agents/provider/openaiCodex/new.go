package openaicodex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

const prefix = "codex@"

type Agent struct {
	httpClient *http.Client
	model      string
	workDir    string

	token *StoredToken
}

func New(model ...string) (*Agent, error) {
	usedModel := provider.Default("codex")
	if len(model) > 0 && strings.HasPrefix(model[0], prefix) {
		usedModel = strings.TrimPrefix(model[0], prefix)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd: %w", err)
	}

	a := &Agent{
		httpClient: &http.Client{Timeout: 10 * time.Minute},
		model:      usedModel,
		workDir:    workDir,
	}

	raw := keychain.Get(tokenKey)
	if raw == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		token, err := a.Login(ctx)
		if err != nil {
			return nil, fmt.Errorf("a.Login: %w", err)
		}
		a.token = token
		return a, nil
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
			ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel2()
			token, err := a.Login(ctx2)
			if err != nil {
				return nil, fmt.Errorf("a.Login: %w", err)
			}
			a.token = token
		}
	}

	return a, nil
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
