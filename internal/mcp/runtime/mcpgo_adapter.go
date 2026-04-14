package runtime

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcptypes "github.com/mark3labs/mcp-go/mcp"

	"persona_agent/internal/config"
)

const (
	defaultClientName    = "persona_agent"
	defaultClientVersion = "0.1.0"
)

type mcpgoClient struct {
	name      string
	timeoutMS int
	client    *mcpclient.Client
}

func newMCPGoClient(ctx context.Context, serverName string, server config.MCPServerConfig) (Client, error) {
	timeoutMS := server.Timeout
	if timeoutMS <= 0 {
		timeoutMS = 30000
	}

	cli, err := buildMCPClient(server, timeoutMS)
	if err != nil {
		return nil, err
	}

	wrapped := &mcpgoClient{name: serverName, timeoutMS: timeoutMS, client: cli}
	if err := wrapped.initialize(ctx); err != nil {
		_ = wrapped.Close()
		return nil, err
	}

	return wrapped, nil
}

func buildMCPClient(server config.MCPServerConfig, timeoutMS int) (*mcpclient.Client, error) {
	httpClient := &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}
	transportName := strings.ToLower(strings.TrimSpace(server.Transport))
	switch transportName {
	case "stdio":
		if strings.TrimSpace(server.Command) == "" {
			return nil, fmt.Errorf("command is required for stdio transport")
		}
		env := make([]string, 0, len(server.Env))
		for k, v := range server.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}

		var options []transport.StdioOption
		if strings.TrimSpace(server.Cwd) != "" {
			cwd := strings.TrimSpace(server.Cwd)
			options = append(options, transport.WithCommandFunc(func(ctx context.Context, command string, env []string, args []string) (*exec.Cmd, error) {
				cmd := exec.CommandContext(ctx, command, args...)
				cmd.Env = append(os.Environ(), env...)
				cmd.Dir = cwd
				return cmd, nil
			}))
		}

		if len(options) == 0 {
			return mcpclient.NewStdioMCPClient(server.Command, env, server.Args...)
		}

		client, err := mcpclient.NewStdioMCPClientWithOptions(server.Command, env, server.Args, options...)
		if err != nil {
			return nil, err
		}
		return client, nil
	case "sse":
		if strings.TrimSpace(server.URL) == "" {
			return nil, fmt.Errorf("url is required for sse transport")
		}
		opts := []transport.ClientOption{
			transport.WithHTTPClient(httpClient),
		}
		if len(server.Headers) > 0 {
			opts = append(opts, transport.WithHeaders(server.Headers))
		}
		return mcpclient.NewSSEMCPClient(server.URL, opts...)
	case "http":
		if strings.TrimSpace(server.URL) == "" {
			return nil, fmt.Errorf("url is required for http transport")
		}
		opts := []transport.StreamableHTTPCOption{
			transport.WithHTTPBasicClient(httpClient),
		}
		if len(server.Headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(server.Headers))
		}
		return mcpclient.NewStreamableHttpClient(server.URL, opts...)
	default:
		return nil, fmt.Errorf("unsupported mcp transport %q", server.Transport)
	}
}

func (c *mcpgoClient) initialize(ctx context.Context) error {
	timedCtx, cancel := withTimeout(ctx, c.timeoutMS)
	defer cancel()

	if err := c.client.Start(timedCtx); err != nil {
		return fmt.Errorf("start mcp client %q: %w", c.name, err)
	}

	initReq := mcptypes.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcptypes.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcptypes.Implementation{Name: defaultClientName, Version: defaultClientVersion}
	initReq.Params.Capabilities = mcptypes.ClientCapabilities{}
	if _, err := c.client.Initialize(timedCtx, initReq); err != nil {
		return fmt.Errorf("initialize mcp client %q: %w", c.name, err)
	}

	return nil
}

func (c *mcpgoClient) ListTools(ctx context.Context) ([]mcptypes.Tool, error) {
	timedCtx, cancel := withTimeout(ctx, c.timeoutMS)
	defer cancel()

	res, err := c.client.ListTools(timedCtx, mcptypes.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []mcptypes.Tool{}, nil
	}
	return res.Tools, nil
}

func (c *mcpgoClient) CallTool(ctx context.Context, toolName string, args map[string]any) (*mcptypes.CallToolResult, error) {
	timedCtx, cancel := withTimeout(ctx, c.timeoutMS)
	defer cancel()

	req := mcptypes.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args
	return c.client.CallTool(timedCtx, req)
}

func (c *mcpgoClient) Close() error {
	return c.client.Close()
}

func withTimeout(ctx context.Context, timeoutMS int) (context.Context, context.CancelFunc) {
	if timeoutMS <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
}
