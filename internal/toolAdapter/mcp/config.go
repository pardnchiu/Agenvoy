package mcp

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type ServerConfig struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type Config struct {
	Servers map[string]ServerConfig `json:"servers,omitempty"`
}

type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeSession Scope = "session"
)

func Load(path string) (Config, error) {
	if path == "" || !go_pkg_filesystem_reader.Exists(path) {
		return Config{Servers: map[string]ServerConfig{}}, nil
	}
	cfg, err := go_pkg_filesystem.ReadJSON[Config](path)
	if err != nil {
		return Config{}, fmt.Errorf("go_pkg_filesystem.ReadJSON: %w", err)
	}
	if cfg.Servers == nil {
		cfg.Servers = map[string]ServerConfig{}
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if cfg.Servers == nil {
		cfg.Servers = map[string]ServerConfig{}
	}
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
	}
	if err := go_pkg_filesystem.WriteJSON(path, cfg, true); err != nil {
		return fmt.Errorf("go_pkg_filesystem.WriteJSON: %w", err)
	}
	return nil
}

func Read(sessionID string) (Config, error) {
	root, err := Load(filesystem.McpPath)
	if err != nil {
		return Config{}, err
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return root, nil
	}
	session, err := Load(filesystem.McpSessionPath(sessionID))
	if err != nil {
		return Config{}, err
	}

	merged := Config{Servers: map[string]ServerConfig{}}
	maps.Copy(merged.Servers, root.Servers)
	maps.Copy(merged.Servers, session.Servers)

	return merged, nil
}

func (c ServerConfig) Expand() ServerConfig {
	out := ServerConfig{
		Command: c.Command,
		URL:     c.URL,
	}
	if len(c.Args) > 0 {
		out.Args = make([]string, len(c.Args))
		for i, a := range c.Args {
			out.Args[i] = os.ExpandEnv(a)
		}
	}
	if len(c.Env) > 0 {
		out.Env = make(map[string]string, len(c.Env))
		for k, v := range c.Env {
			out.Env[k] = os.ExpandEnv(v)
		}
	}
	if len(c.Headers) > 0 {
		out.Headers = make(map[string]string, len(c.Headers))
		for k, v := range c.Headers {
			out.Headers[k] = normalizeHeaderValue(k, os.ExpandEnv(v))
		}
	}
	return out
}

func normalizeHeaderValue(key, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.EqualFold(strings.TrimSpace(key), "Authorization") {
		return value
	}
	if len(strings.Fields(value)) > 1 {
		return value
	}
	return "Bearer " + value
}

func (c ServerConfig) IsHTTP() bool {
	return strings.TrimSpace(c.URL) != ""
}

func (c ServerConfig) IsStdio() bool {
	return strings.TrimSpace(c.Command) != ""
}
