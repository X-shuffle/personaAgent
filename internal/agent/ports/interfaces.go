package ports

import (
	"context"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
)

// ToolCaller is the outbound port used by orchestrator to execute one MCP tool call.
type ToolCaller interface {
	CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (*mcptypes.CallToolResult, error)
}

// ToolCatalogProvider is the outbound port used by orchestrator to fetch available MCP tools.
type ToolCatalogProvider interface {
	ToolCatalog() map[string][]mcptypes.Tool
}
