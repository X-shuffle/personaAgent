package runtime

import (
	"context"
	"errors"
	"testing"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"persona_agent/internal/config"
)

type fakeClient struct {
	tools    []mcptypes.Tool
	callResp *mcptypes.CallToolResult
	callErr  error
	closeErr error
}

func (f *fakeClient) ListTools(context.Context) ([]mcptypes.Tool, error) {
	return f.tools, nil
}

func (f *fakeClient) CallTool(context.Context, string, map[string]any) (*mcptypes.CallToolResult, error) {
	if f.callErr != nil {
		return nil, f.callErr
	}
	if f.callResp != nil {
		return f.callResp, nil
	}
	return &mcptypes.CallToolResult{}, nil
}

func (f *fakeClient) Close() error {
	return f.closeErr
}

func TestManagerStartAndCallTool(t *testing.T) {
	manager := NewManagerWithFactory(zap.NewNop(), func(ctx context.Context, serverName string, server config.MCPServerConfig) (Client, error) {
		return &fakeClient{tools: []mcptypes.Tool{{Name: "ping"}}}, nil
	})

	report := manager.Start(context.Background(), map[string]config.MCPServerConfig{
		"alpha": {Transport: "stdio", Command: "dummy"},
	})
	if report.Configured != 1 || report.Connected != 1 || len(report.FailedByServer) != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.ToolCountByServer["alpha"] != 1 {
		t.Fatalf("expected alpha tool count 1, got %d", report.ToolCountByServer["alpha"])
	}

	_, err := manager.CallTool(context.Background(), "alpha", "ping", map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
}

func TestManagerStartFailSoft(t *testing.T) {
	manager := NewManagerWithFactory(zap.NewNop(), func(ctx context.Context, serverName string, server config.MCPServerConfig) (Client, error) {
		if serverName == "bad" {
			return nil, errors.New("connect failed")
		}
		return &fakeClient{}, nil
	})

	report := manager.Start(context.Background(), map[string]config.MCPServerConfig{
		"bad":  {Transport: "http", URL: "https://bad"},
		"good": {Transport: "stdio", Command: "dummy"},
	})

	if report.Configured != 2 {
		t.Fatalf("expected configured 2, got %d", report.Configured)
	}
	if report.Connected != 1 {
		t.Fatalf("expected connected 1, got %d", report.Connected)
	}
	if len(report.FailedByServer) != 1 {
		t.Fatalf("expected failed map size 1, got %d", len(report.FailedByServer))
	}
	if _, ok := report.FailedByServer["bad"]; !ok {
		t.Fatalf("expected bad server failure")
	}
}

func TestManagerCloseAggregatesErrors(t *testing.T) {
	manager := NewManagerWithFactory(zap.NewNop(), func(ctx context.Context, serverName string, server config.MCPServerConfig) (Client, error) {
		if serverName == "x" {
			return &fakeClient{closeErr: errors.New("close x")}, nil
		}
		return &fakeClient{closeErr: errors.New("close y")}, nil
	})

	_ = manager.Start(context.Background(), map[string]config.MCPServerConfig{
		"x": {Transport: "stdio", Command: "dummy"},
		"y": {Transport: "stdio", Command: "dummy"},
	})

	err := manager.Close()
	if err == nil {
		t.Fatalf("expected close error")
	}
	if manager.ConnectedServerCount() != 0 {
		t.Fatalf("expected no connected servers after close")
	}
}

func TestManagerToolCatalogPopulatedOnStart(t *testing.T) {
	manager := NewManagerWithFactory(zap.NewNop(), func(ctx context.Context, serverName string, server config.MCPServerConfig) (Client, error) {
		return &fakeClient{tools: []mcptypes.Tool{{Name: "ping"}, {Name: "weather"}}}, nil
	})

	_ = manager.Start(context.Background(), map[string]config.MCPServerConfig{
		"alpha": {Transport: "stdio", Command: "dummy"},
	})

	catalog := manager.ToolCatalog()
	tools := catalog["alpha"]
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools in catalog, got %d", len(tools))
	}
	if tools[0].Name != "ping" || tools[1].Name != "weather" {
		t.Fatalf("unexpected tools in catalog: %+v", tools)
	}
}

func TestManagerToolCatalogExcludesFailedServers(t *testing.T) {
	manager := NewManagerWithFactory(zap.NewNop(), func(ctx context.Context, serverName string, server config.MCPServerConfig) (Client, error) {
		if serverName == "bad" {
			return nil, errors.New("connect failed")
		}
		return &fakeClient{tools: []mcptypes.Tool{{Name: "ping"}}}, nil
	})

	_ = manager.Start(context.Background(), map[string]config.MCPServerConfig{
		"bad":  {Transport: "http", URL: "https://bad"},
		"good": {Transport: "stdio", Command: "dummy"},
	})

	catalog := manager.ToolCatalog()
	if _, ok := catalog["bad"]; ok {
		t.Fatalf("bad server should not be in catalog")
	}
	if len(catalog["good"]) != 1 {
		t.Fatalf("good server should have one tool")
	}
}

func TestManagerCloseClearsToolCatalog(t *testing.T) {
	manager := NewManagerWithFactory(zap.NewNop(), func(ctx context.Context, serverName string, server config.MCPServerConfig) (Client, error) {
		return &fakeClient{tools: []mcptypes.Tool{{Name: "ping"}}}, nil
	})

	_ = manager.Start(context.Background(), map[string]config.MCPServerConfig{
		"alpha": {Transport: "stdio", Command: "dummy"},
	})
	if len(manager.ToolCatalog()) != 1 {
		t.Fatalf("expected non-empty catalog before close")
	}

	_ = manager.Close()
	if len(manager.ToolCatalog()) != 0 {
		t.Fatalf("expected empty catalog after close")
	}
}
