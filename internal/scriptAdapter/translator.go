package scriptAdapter

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Translator struct {
	scripts map[string]*Script
}

type Script struct {
	Doc        ScriptDoc
	scriptPath string
	language   string
}

type ScriptDoc struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

var runtimeMap = map[string]string{
	"javascript": "node",
	"python":     "python3",
}

func New() *Translator {
	return &Translator{
		scripts: make(map[string]*Script),
	}
}

func (t *Translator) Scan(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("os.ReadDir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		toolDir := filepath.Join(dir, entry.Name())
		doc, scriptPath, lang, err := loadDir(toolDir)
		if err != nil {
			slog.Warn("loadDir",
				slog.String("err", err.Error()))
			continue
		}
		t.scripts[doc.Name] = &Script{
			Doc:        *doc,
			scriptPath: scriptPath,
			language:   lang,
		}
	}
	return nil
}

func (t *Translator) IsExist(name string) bool {
	key := strings.TrimPrefix(name, "script_")
	_, ok := t.scripts[key]
	return ok
}

func (t *Translator) GetTools() []map[string]any {
	tools := make([]map[string]any, 0, len(t.scripts))
	for _, script := range t.scripts {
		params := json.RawMessage(`{"type":"object","properties":{}}`)
		if len(script.Doc.Parameters) > 0 {
			params = script.Doc.Parameters
		}
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "script_" + script.Doc.Name,
				"description": script.Doc.Description,
				"parameters":  params,
			},
		})
	}
	return tools
}

func loadDir(dir string) (*ScriptDoc, string, string, error) {
	content, err := os.ReadFile(filepath.Join(dir, "tool.json"))
	if err != nil {
		return nil, "", "", fmt.Errorf("os.ReadFile: %w", err)
	}

	var doc ScriptDoc
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, "", "", fmt.Errorf("json.Unmarshal: %w", err)
	}
	if strings.TrimSpace(doc.Name) == "" {
		return nil, "", "", fmt.Errorf("script tool name is required")
	}
	if strings.Contains(doc.Name, "/") || doc.Name == ".." {
		return nil, "", "", fmt.Errorf("invalid script tool name: %q", doc.Name)
	}

	scriptPath, lang, err := findScript(dir)
	if err != nil {
		return nil, "", "", err
	}
	return &doc, scriptPath, lang, nil
}

func findScript(dir string) (string, string, error) {
	candidates := []struct{ name, lang string }{
		{"script.js", "javascript"},
		{"script.py", "python"},
	}
	for _, c := range candidates {
		p := filepath.Join(dir, c.name)
		if _, err := os.Stat(p); err == nil {
			return p, c.lang, nil
		}
	}
	return "", "", fmt.Errorf("script not found in %s", dir)
}
