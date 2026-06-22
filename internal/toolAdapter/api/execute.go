package apiAdapter

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"strings"
	"time"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"
)

func (a *Adapter) Execute(ctx context.Context, name string, params map[string]any) (string, error) {
	key := strings.TrimPrefix(name, a.prefix)
	doc, ok := a.apis[key]
	if !ok {
		return "", fmt.Errorf("api tool not found: %s", name)
	}

	if err := a.checkParams(doc, params); err != nil {
		return "", err
	}

	result, err := a.send(ctx, key, doc, params)
	if err != nil {
		return "", fmt.Errorf("t.send: %w", err)
	}

	return result, nil
}

func (a *Adapter) checkParams(doc *Document, params map[string]any) error {
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

func (a *Adapter) send(ctx context.Context, key string, doc *Document, params map[string]any) (string, error) {
	apiPath := replaceParams(doc, params)

	header := make(map[string]string, len(doc.Endpoint.Headers)+1)
	maps.Copy(header, doc.Endpoint.Headers)
	if doc.Auth != nil && *doc.Auth != (DocumentAuth{}) {
		if err := appendAuth(header, doc.Auth); err != nil {
			return "", err
		}
	}

	sec := doc.Endpoint.Timeout
	if sec <= 0 {
		sec = 60
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(sec)*time.Second)
	defer cancel()

	name := a.prefix + key
	resultCh := make(chan struct {
		body string
		err  error
	}, 1)

	go func() {
		var (
			body string
			err  error
		)
		switch doc.Endpoint.Method {
		case "GET":
			query := url.Values{}
			for k, v := range doc.Endpoint.Query {
				query.Set(k, v)
			}
			for k, v := range params {
				query.Set(k, fmt.Sprintf("%v", v))
			}
			if len(query) > 0 {
				apiPath = apiPath + "?" + query.Encode()
			}

			body, _, err = go_pkg_http.GET[string](reqCtx, a.client, apiPath, header)
		case "POST":
			body, _, err = go_pkg_http.POST[string](reqCtx, a.client, apiPath, header, params, doc.Endpoint.ContentType)
		case "PUT":
			body, _, err = go_pkg_http.PUT[string](reqCtx, a.client, apiPath, header, params, doc.Endpoint.ContentType)
		case "PATCH":
			body, _, err = go_pkg_http.PATCH[string](reqCtx, a.client, apiPath, header, params, doc.Endpoint.ContentType)
		case "DELETE":
			body, _, err = go_pkg_http.DELETE[string](reqCtx, a.client, apiPath, header, params, doc.Endpoint.ContentType)
		default:
			err = fmt.Errorf("unsupported method: %s", doc.Endpoint.Method)
		}
		resultCh <- struct {
			body string
			err  error
		}{body, err}
	}()

	// * every 30s print for checking health
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case r := <-resultCh:
			return r.body, r.err

		case <-ticker.C:
			slog.Warn("running",
				slog.String("name", name),
				slog.String("elapsed", fmt.Sprintf("%ds/%ds", int(time.Since(start).Seconds()), sec)))

		case <-reqCtx.Done():
			return "", reqCtx.Err()
		}
	}
}

// * https://example.com/post/{id}
func replaceParams(doc *Document, params map[string]any) string {
	apiPath := doc.Endpoint.URL

	for k, v := range params {
		placeholder := "{" + k + "}"
		if strings.Contains(apiPath, placeholder) {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				delete(params, k)
				continue
			}

			apiPath = strings.ReplaceAll(apiPath, placeholder, url.PathEscape(val))
			delete(params, k)
		}
	}

	for strings.Contains(apiPath, "{") {
		start := strings.Index(apiPath, "{")
		end := strings.Index(apiPath, "}")
		if end < start {
			break
		}

		newStart := start
		if newStart > 0 && apiPath[newStart-1] == '/' {
			newStart--
		}
		apiPath = apiPath[:newStart] + apiPath[end+1:]
	}
	return apiPath
}

func appendAuth(header map[string]string, auth *DocumentAuth) error {
	if auth.Env == "" {
		return fmt.Errorf("env key is required")
	}

	// * keychain already contain keychain and env check
	value := keychain.Get(auth.Env)
	if value == "" {
		return fmt.Errorf("%q not set", auth.Env)
	}

	switch auth.Type {
	case "bearer":
		header["Authorization"] = "Bearer " + value

	case "apikey":
		key := auth.Header
		if key == "" {
			key = "X-API-Key"
		}
		header[key] = value

	case "basic":
		header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(value))

	default:
		return fmt.Errorf("unsupported auth: %s", auth.Type)
	}
	return nil
}
