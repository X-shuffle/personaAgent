package runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"go.uber.org/zap"

	"persona_agent/internal/config"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
)

type Client interface {
	ListTools(ctx context.Context) ([]mcptypes.Tool, error)
	CallTool(ctx context.Context, toolName string, args map[string]any) (*mcptypes.CallToolResult, error)
	Close() error
}

type clientFactory func(ctx context.Context, serverName string, server config.MCPServerConfig) (Client, error)

type StartReport struct {
	Configured      int
	Connected       int
	FailedByServer  map[string]error
	ToolCountByServer map[string]int
}

type Manager struct {
	logger  *zap.Logger
	factory clientFactory

	mu      sync.RWMutex
	clients map[string]Client
}

func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger:  logger,
		factory: newMCPGoClient,
		clients: map[string]Client{},
	}
}

func NewManagerWithFactory(logger *zap.Logger, factory clientFactory) *Manager {
	if factory == nil {
		factory = newMCPGoClient
	}
	return &Manager{
		logger:  logger,
		factory: factory,
		clients: map[string]Client{},
	}
}

func (m *Manager) Start(ctx context.Context, servers map[string]config.MCPServerConfig) StartReport {
	report := StartReport{
		Configured:        len(servers),
		FailedByServer:    map[string]error{},
		ToolCountByServer: map[string]int{},
	}
	if len(servers) == 0 {
		return report
	}

	names := sortedServerNames(servers)
	for _, name := range names {
		server := servers[name]
		cli, err := m.factory(ctx, name, server)
		if err != nil {
			report.FailedByServer[name] = err
			if m.logger != nil {
				m.logger.Warn("mcp server connect failed", zap.String("server", name), zap.Error(err))
			}
			continue
		}

		tools, err := cli.ListTools(ctx)
		if err != nil {
			report.FailedByServer[name] = fmt.Errorf("list tools: %w", err)
			_ = cli.Close()
			if m.logger != nil {
				m.logger.Warn("mcp server list tools failed", zap.String("server", name), zap.Error(err))
			}
			continue
		}

		m.mu.Lock()
		m.clients[name] = cli
		m.mu.Unlock()

		report.Connected++
		report.ToolCountByServer[name] = len(tools)
	}

	return report
}

func (m *Manager) ConnectedServerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (*mcptypes.CallToolResult, error) {
	m.mu.RLock()
	cli, ok := m.clients[serverName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("mcp server %q not connected", serverName)
	}

	return cli.CallTool(ctx, toolName, args)
}

func (m *Manager) Close() error {
	m.mu.Lock()
	clients := m.clients
	m.clients = map[string]Client{}
	m.mu.Unlock()

	var joined error
	for name, cli := range clients {
		if err := cli.Close(); err != nil {
			joined = errors.Join(joined, fmt.Errorf("close mcp server %q: %w", name, err))
		}
	}

	return joined
}

func sortedServerNames(servers map[string]config.MCPServerConfig) []string {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
