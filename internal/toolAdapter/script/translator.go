package scriptAdapter

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

type Translator struct {
	prefix  string
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
	AlwaysAllow bool            `json:"always_allow,omitempty"`
	Concurrent  bool            `json:"concurrent,omitempty"`
	Timeout     int             `json:"timeout,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

var runtimeMap = map[string]string{
	"javascript": "node",
	"python":     "python3",
}

func New(prefix string) *Translator {
	return &Translator{
		prefix:  prefix,
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
		if strings.HasPrefix(entry.Name(), "_") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		toolDir := filepath.Join(dir, entry.Name())
		doc, scriptPath, lang, err := loadDir(toolDir)
		if err != nil {
			slog.Warn("loadDir",
				slog.String("error", err.Error()))
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
	key := strings.TrimPrefix(name, t.prefix)
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
				"name":        t.prefix + script.Doc.Name,
				"description": script.Doc.Description,
				"parameters":  params,
			},
		})
	}
	return tools
}

func (t *Translator) AlwaysAllowNames() []string {
	names := make([]string, 0, len(t.scripts))
	for _, script := range t.scripts {
		if script.Doc.AlwaysAllow {
			names = append(names, t.prefix+script.Doc.Name)
		}
	}
	return names
}

func (t *Translator) ConcurrentNames() []string {
	names := make([]string, 0, len(t.scripts))
	for _, script := range t.scripts {
		names = append(names, t.prefix+script.Doc.Name)
	}
	return names
}

func (t *Translator) Timeouts() map[string]int {
	out := make(map[string]int, len(t.scripts))
	for _, script := range t.scripts {
		if script.Doc.Timeout > 0 {
			out[t.prefix+script.Doc.Name] = script.Doc.Timeout
		}
	}
	return out
}

func loadDir(dir string) (*ScriptDoc, string, string, error) {
	doc, err := go_pkg_filesystem.ReadJSON[ScriptDoc](filepath.Join(dir, "tool.json"))
	if err != nil {
		return nil, "", "", fmt.Errorf("go_pkg_filesystem.ReadJSON: %w", err)
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
		if go_pkg_filesystem_reader.Exists(p) {
			return p, c.lang, nil
		}
	}
	return "", "", fmt.Errorf("script not found in %s", dir)
}
