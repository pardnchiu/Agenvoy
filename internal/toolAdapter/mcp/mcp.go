package mcp

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
)

type MCP struct {
	mu      sync.Mutex
	clients map[string]Client
}

var (
	managerMu sync.RWMutex
	manager   *MCP
)

func SetManager(m *MCP) {
	managerMu.Lock()
	defer managerMu.Unlock()
	manager = m
}

func Manager() *MCP {
	managerMu.RLock()
	defer managerMu.RUnlock()
	return manager
}

type ServerInfo struct {
	Name      string
	Transport string
	Connected bool
}

func New(ctx context.Context, sessionID string) (*MCP, error) {
	cfg, err := Read(sessionID)
	if err != nil {
		return nil, err
	}

	mcp := &MCP{
		clients: map[string]Client{},
	}

	for _, key := range slices.Sorted(maps.Keys(cfg.Servers)) {
		client, err := newClient(ctx, key, cfg.Servers[key])
		if err != nil {
			slog.Warn("newClient",
				slog.String("server", key),
				slog.String("error", err.Error()))
			continue
		}
		mcp.clients[key] = client
	}
	return mcp, nil
}

func (m *MCP) Status(sessionID string) []ServerInfo {
	if m == nil {
		return nil
	}
	cfg, err := Read(sessionID)
	if err != nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	list := make([]ServerInfo, 0, len(cfg.Servers))
	for _, name := range slices.Sorted(maps.Keys(cfg.Servers)) {
		s := cfg.Servers[name]
		transport := "stdio"
		if s.Expand().IsHTTP() {
			transport = "streamable-http"
		}
		_, connected := m.clients[name]
		list = append(list, ServerInfo{
			Name:      name,
			Transport: transport,
			Connected: connected,
		})
	}
	return list
}

func (m *MCP) Reconnect(ctx context.Context, sessionID string) error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	for _, c := range m.clients {
		_ = c.Close()
	}
	m.clients = map[string]Client{}
	m.mu.Unlock()

	toolRegister.RemoveByPrefix("mcp__")

	cfg, err := Read(strings.TrimSpace(sessionID))
	if err != nil {
		return err
	}

	m.mu.Lock()
	for _, key := range slices.Sorted(maps.Keys(cfg.Servers)) {
		client, err := newClient(ctx, key, cfg.Servers[key])
		if err != nil {
			slog.Warn("mcp reconnect newClient",
				slog.String("server", key),
				slog.String("error", err.Error()))
			continue
		}
		m.clients[key] = client
	}
	m.mu.Unlock()

	m.RegisterAll(ctx)
	return nil
}

func (m *MCP) RegisterAll(ctx context.Context) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.clients {
		tools, err := client.List(ctx)
		if err != nil {
			slog.Warn("client.List",
				slog.String("server", name),
				slog.String("error", err.Error()))
			continue
		}

		for _, tool := range tools {
			def, ok := tool.getDef(name, client)
			if !ok {
				slog.Warn("tool.getDef",
					slog.String("server", name),
					slog.String("tool", tool.Name))
				continue
			}
			toolRegister.Regist(def)
		}
	}
}

func (m *MCP) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		_ = client.Close()
	}
	m.clients = map[string]Client{}
}
