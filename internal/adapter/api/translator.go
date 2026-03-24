package apiAdapter

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Translator struct {
	apis   map[string]*APIDocumentData
	client *http.Client
}

func New() *Translator {
	return &Translator{
		apis:   make(map[string]*APIDocumentData),
		client: &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()},
	}
}

func (t *Translator) Load(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("os.ReadDir: %w", err)
	}

	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		apiPath := filepath.Join(path, entry.Name())
		doc, err := t.load(apiPath)
		if err != nil {
			slog.Warn("failed to load API",
				slog.String("path", apiPath),
				slog.String("error", err.Error()))
			continue
		}
		if doc == nil {
			continue
		}

		t.apis[doc.Name] = doc
		loaded++
	}
	return nil
}

func (t *Translator) load(path string) (*APIDocumentData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("os.ReadFile: %w", err)
	}

	var doc APIDocumentData
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}

	if err := t.check(&doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

func (t *Translator) check(doc *APIDocumentData) error {
	if doc.Name == "" {
		return fmt.Errorf("name is required")
	}

	if doc.Description == "" {
		return fmt.Errorf("description is required")
	}

	if doc.Endpoint.URL == "" {
		return fmt.Errorf("endpoint.url is required")
	}

	if doc.Endpoint.Method == "" {
		return fmt.Errorf("endpoint.method is required")
	}

	doc.Endpoint.Method = strings.ToUpper(doc.Endpoint.Method)
	switch doc.Endpoint.Method {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
	default:
		return fmt.Errorf("unsupported method")
	}

	if doc.Endpoint.ContentType == "" {
		doc.Endpoint.ContentType = "json"
	}

	if doc.Response.Format == "" {
		doc.Response.Format = "json"
	}

	return nil
}

func (t *Translator) LoadFS(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("fs.ReadDir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := fs.ReadFile(fsys, fmt.Sprintf("%s/%s", dir, entry.Name()))
		if err != nil {
			slog.Warn("fs.ReadFile",
				slog.String("error", err.Error()))
			continue
		}

		var doc APIDocumentData
		if err := json.Unmarshal(data, &doc); err != nil {
			slog.Warn("json.Unmarshal",
				slog.String("error", err.Error()))
			continue
		}

		if err := t.check(&doc); err != nil {
			slog.Warn("t.check",
				slog.String("error", err.Error()))
			continue
		}

		t.apis[doc.Name] = &doc
	}
	return nil
}

func (t *Translator) IsExist(name string) bool {
	key := strings.TrimPrefix(name, "api_")
	_, ok := t.apis[key]
	return ok
}

func (t *Translator) GetTools() []map[string]any {
	tools := make([]map[string]any, 0, len(t.apis))
	for _, api := range t.apis {
		tools = append(tools, api.translate())
	}
	return tools
}
