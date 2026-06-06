package tool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session/config"
)

func Register() {
	cfg, err := config.Load()
	if err != nil || cfg == nil || !cfg.KuradbEnabled {
		return
	}
	registRagListDB()
	registSearchRag()
}

var (
	ragClient = &http.Client{Timeout: 15 * time.Second}
)

func kuradbGet(ctx context.Context, path string, query url.Values) (string, error) {
	base, err := filesystem.GetKuradbEndpoint()
	if err != nil {
		return "", fmt.Errorf("kuradb not running: %w", err)
	}

	full := strings.TrimRight(base, "/") + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return "", fmt.Errorf("http.NewRequestWithContext: %w", err)
	}
	resp, err := ragClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ragClient.Do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("io.ReadAll: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("kuradb %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}
