package apiAdapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

const (
	apiMinGap = 1 * time.Second
)

var (
	apiSlotMu   sync.Mutex
	apiKeyMutex = make(map[string]*sync.Mutex)
	apiLastCall = make(map[string]time.Time)
)

func (t *Translator) Execute(ctx context.Context, name string, params map[string]any) (string, error) {
	key := strings.TrimPrefix(name, "api_")
	doc, ok := t.apis[key]
	if !ok {
		return "", fmt.Errorf("api tool not found: %s", name)
	}

	normalizeAliasParams(doc, params)

	if err := t.checkRequireds(doc, params); err != nil {
		return "", fmt.Errorf("t.checkRequireds: %w", err)
	}

	if err := reserveAPISlot(ctx, key); err != nil {
		return "", err
	}

	result, err := t.send(ctx, key, doc, params)
	if err != nil {
		return "", fmt.Errorf("t.send: %w", err)
	}

	return result, nil
}

func reserveAPISlot(ctx context.Context, key string) error {
	apiSlotMu.Lock()
	mu, ok := apiKeyMutex[key]
	if !ok {
		mu = &sync.Mutex{}
		apiKeyMutex[key] = mu
	}
	apiSlotMu.Unlock()

	mu.Lock()
	defer mu.Unlock()

	if wait := apiMinGap - time.Since(apiLastCall[key]); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	apiLastCall[key] = time.Now()
	return nil
}

func normalizeAliasParams(doc *APIDocumentData, params map[string]any) {
	aliasMap := map[string][]string{
		"currency": {"base", "from"},
	}

	for canonical, aliases := range aliasMap {
		if _, ok := doc.Parameters[canonical]; !ok {
			continue
		}
		if value, ok := params[canonical]; ok && value != nil {
			continue
		}
		for _, alias := range aliases {
			if value, ok := params[alias]; ok && value != nil {
				params[canonical] = value
				break
			}
		}
	}
}

func (t *Translator) checkRequireds(doc *APIDocumentData, params map[string]any) error {
	for name, schema := range doc.Parameters {
		if _, exists := params[name]; !exists {
			if schema.Required {
				return fmt.Errorf("%q is required", name)
			}
			if schema.Default != nil {
				params[name] = schema.Default
			}
		}
	}
	return nil
}

func (t *Translator) send(ctx context.Context, key string, doc *APIDocumentData, params map[string]any) (string, error) {
	var (
		req *http.Request
		err error
	)

	switch doc.Endpoint.ContentType {
	case "form":
		req, err = t.FormDataRequest(doc, params)
	default:
		req, err = t.JSONRequest(doc, params)
	}
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	for k, v := range doc.Endpoint.Headers {
		req.Header.Set(k, v)
	}

	// * add auth to header, ex. bearer token, api key, basic auth
	if doc.Auth != nil && *doc.Auth != (APIDocumentAuthData{}) {
		if err := t.insetAuth(req, doc.Auth); err != nil {
			return "", err
		}
	}

	timeoutSec := doc.Endpoint.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	timeout := time.Duration(timeoutSec) * time.Second

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req = req.WithContext(reqCtx)

	resp, err := t.doWithProgress(reqCtx, "api_"+key, req, timeoutSec)
	if err != nil {
		return "", fmt.Errorf("client.Do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("io.ReadAll: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resp.StatusCode: %d", resp.StatusCode)
	}

	if doc.Response.Format == "json" {
		var data any
		if err := json.Unmarshal(body, &data); err == nil {
			output, err := json.Marshal(data)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(output), nil
		}
	}

	return string(body), nil
}

func (t *Translator) insetAuth(req *http.Request, auth *APIDocumentAuthData) error {
	if auth.Env == "" {
		return fmt.Errorf("auth.env is required")
	}

	value := keychain.Get(auth.Env)
	if value == "" {
		return fmt.Errorf("%q not set", auth.Env)
	}

	switch auth.Type {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+value)

	case "apikey":
		header := auth.Header
		if header == "" {
			header = "X-API-Key"
		}
		req.Header.Set(header, value)

	case "basic":
		encoded := base64.StdEncoding.EncodeToString([]byte(value))
		req.Header.Set("Authorization", "Basic "+encoded)

	default:
		return fmt.Errorf("unsupported auth: %s", auth.Type)
	}

	return nil
}

type httpResult struct {
	resp *http.Response
	err  error
}

func (t *Translator) doWithProgress(ctx context.Context, name string, req *http.Request, timeoutSec int) (*http.Response, error) {
	done := make(chan httpResult, 1)
	go func() {
		resp, err := t.client.Do(req)
		done <- httpResult{resp, err}
	}()

	start := time.Now()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case r := <-done:
			return r.resp, r.err
		case <-ticker.C:
			slog.Warn("running",
				slog.String("name", name),
				slog.String("elapsed", fmt.Sprintf("%ds/%ds", int(time.Since(start).Seconds()), timeoutSec)))
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
