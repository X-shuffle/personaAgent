package runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"persona_agent/internal/config"
)

// Client 抽象单个 MCP server 的最小能力集。
// Manager 通过该接口解耦底层实现，便于替换与测试。
type Client interface {
	ListTools(ctx context.Context) ([]mcptypes.Tool, error)
	CallTool(ctx context.Context, toolName string, args map[string]any) (*mcptypes.CallToolResult, error)
	Close() error
}

type clientFactory func(ctx context.Context, serverName string, server config.MCPServerConfig) (Client, error)

// StartReport 描述一次 Start 过程的连接结果与工具发现统计。
type StartReport struct {
	Configured       int
	Connected        int
	FailedByServer   map[string]error
	ToolCountByServer map[string]int
}

// Manager 负责 MCP 客户端生命周期管理、工具目录缓存与工具调用分发。
type Manager struct {
	logger  *zap.Logger
	factory clientFactory

	mu            sync.RWMutex
	clients       map[string]Client
	toolsByServer map[string][]mcptypes.Tool
}

// NewManager 创建默认 manager（底层 client 使用 mcp-go 实现）。
// logger 为空会直接 panic，以保证日志路径无判空分支。
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		panic("mcp runtime manager logger is nil")
	}
	return &Manager{
		logger:        logger,
		factory:       newMCPGoClient,
		clients:       map[string]Client{},
		toolsByServer: map[string][]mcptypes.Tool{},
	}
}

// NewManagerWithFactory 创建可注入工厂的 manager，主要用于测试替身。
// logger 为空会直接 panic。
func NewManagerWithFactory(logger *zap.Logger, factory clientFactory) *Manager {
	if logger == nil {
		panic("mcp runtime manager logger is nil")
	}
	if factory == nil {
		factory = newMCPGoClient
	}
	return &Manager{
		logger:        logger,
		factory:       factory,
		clients:       map[string]Client{},
		toolsByServer: map[string][]mcptypes.Tool{},
	}
}

// Start 连接所有启用的 MCP server，并预拉取工具列表用于后续 function-calling。
// 失败按 server 维度记录，不会因为单点失败中断整体启动。
func (m *Manager) Start(ctx context.Context, servers map[string]config.MCPServerConfig) StartReport {
	report := StartReport{
		Configured:        len(servers),
		FailedByServer:    map[string]error{},
		ToolCountByServer: map[string]int{},
	}

	m.mu.Lock()
	m.clients = map[string]Client{}
	m.toolsByServer = map[string][]mcptypes.Tool{}
	m.mu.Unlock()

	if len(servers) == 0 {
		return report
	}

	names := sortedServerNames(servers)
	for _, name := range names {
		server := servers[name]
		cli, err := m.factory(ctx, name, server)
		if err != nil {
			report.FailedByServer[name] = err
			m.logger.Warn("mcp server connect failed", zap.String("server", name), zap.Error(err))
			continue
		}

		tools, err := cli.ListTools(ctx)
		if err != nil {
			report.FailedByServer[name] = fmt.Errorf("list tools: %w", err)
			_ = cli.Close()
			m.logger.Warn("mcp server list tools failed", zap.String("server", name), zap.Error(err))
			continue
		}
		toolNames := make([]string, 0, len(tools))
		for _, tool := range tools {
			toolName := strings.TrimSpace(tool.Name)
			if toolName == "" {
				toolName = "(unnamed)"
			}
			toolNames = append(toolNames, toolName)
		}
		sort.Strings(toolNames)
		m.logger.Debug("mcp server tools discovered", zap.String("server", name), zap.Int("tool_count", len(tools)), zap.Strings("tools", toolNames))

		m.mu.Lock()
		m.clients[name] = cli
		m.toolsByServer[name] = cloneTools(tools)
		m.mu.Unlock()

		report.Connected++
		report.ToolCountByServer[name] = len(tools)
	}

	return report
}

// ToolCatalog 返回当前已连接 server 的工具快照副本，避免外部修改内部状态。
func (m *Manager) ToolCatalog() map[string][]mcptypes.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	catalog := make(map[string][]mcptypes.Tool, len(m.toolsByServer))
	for server, tools := range m.toolsByServer {
		catalog[server] = cloneTools(tools)
	}
	return catalog
}

func (m *Manager) ConnectedServerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// CallTool 将调用分发到指定 server 的客户端。
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (*mcptypes.CallToolResult, error) {
	m.mu.RLock()
	cli, ok := m.clients[serverName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("mcp server %q not connected", serverName)
	}

	return cli.CallTool(ctx, toolName, args)
}

// Close 关闭全部已连接客户端，并聚合返回关闭错误。
func (m *Manager) Close() error {
	m.mu.Lock()
	clients := m.clients
	m.clients = map[string]Client{}
	m.toolsByServer = map[string][]mcptypes.Tool{}
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

func cloneTools(in []mcptypes.Tool) []mcptypes.Tool {
	if len(in) == 0 {
		return nil
	}
	out := make([]mcptypes.Tool, len(in))
	copy(out, in)
	return out
}
