package apiAdapter

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func (t *Translator) Execute(name string, params map[string]any) (string, error) {
	key := strings.TrimPrefix(name, "api_")
	doc, ok := t.apis[key]
	if !ok {
		return "", fmt.Errorf("not found: %s", name)
	}

	if err := t.checkRequireds(doc, params); err != nil {
		return "", err
	}

	result, err := t.send(doc, params)
	if err != nil {
		return "", fmt.Errorf("t.send: %w", err)
	}

	return result, nil
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

func (t *Translator) send(doc *APIDocumentData, params map[string]any) (string, error) {
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

	timeout := doc.Endpoint.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	t.client.Timeout = time.Duration(timeout) * time.Second

	resp, err := t.client.Do(req)
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

	value := os.Getenv(auth.Env)
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
