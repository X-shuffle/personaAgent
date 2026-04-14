package runtime

import (
	"testing"

	"persona_agent/internal/config"
)

func TestBuildMCPClientValidation(t *testing.T) {
	_, err := buildMCPClient(config.MCPServerConfig{Transport: "stdio", Command: ""}, 1000)
	if err == nil {
		t.Fatalf("expected stdio command validation error")
	}

	_, err = buildMCPClient(config.MCPServerConfig{Transport: "http", URL: ""}, 1000)
	if err == nil {
		t.Fatalf("expected http url validation error")
	}

	_, err = buildMCPClient(config.MCPServerConfig{Transport: "sse", URL: ""}, 1000)
	if err == nil {
		t.Fatalf("expected sse url validation error")
	}

	_, err = buildMCPClient(config.MCPServerConfig{Transport: "unknown"}, 1000)
	if err == nil {
		t.Fatalf("expected unsupported transport error")
	}
}
