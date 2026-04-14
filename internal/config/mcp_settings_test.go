package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMCPSettings_OptionalMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	all, active, err := loadMCPSettings(path, false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(all) != 0 || len(active) != 0 {
		t.Fatalf("expected empty maps, got all=%d active=%d", len(all), len(active))
	}
}

func TestLoadMCPSettings_RequiredMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	_, _, err := loadMCPSettings(path, true)
	if err == nil {
		t.Fatalf("expected error for missing required file")
	}
}

func TestLoadMCPSettings_ValidAndDisabledFiltering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_settings.json")
	content := `{
  "mcpServers": {
    "stdio_ok": {
      "command": "node",
      "args": ["server.js"],
      "transport": "stdio"
    },
    "http_disabled": {
      "transport": "http",
      "url": "https://example.com/mcp",
      "disabled": true
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	all, active, err := loadMCPSettings(path, true)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected all=2, got %d", len(all))
	}
	if len(active) != 1 {
		t.Fatalf("expected active=1, got %d", len(active))
	}
	if _, ok := active["stdio_ok"]; !ok {
		t.Fatalf("expected stdio_ok to be active")
	}
	if all["stdio_ok"].Timeout != defaultMCPTimeoutMS {
		t.Fatalf("expected default timeout %d, got %d", defaultMCPTimeoutMS, all["stdio_ok"].Timeout)
	}
}

func TestLoadMCPSettings_InvalidTransportCombination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_settings.json")
	content := `{
  "mcpServers": {
    "bad": {
      "transport": "stdio",
      "url": "https://example.com/mcp"
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	_, _, err := loadMCPSettings(path, true)
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestLoadMCPSettings_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_settings.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":`), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	_, _, err := loadMCPSettings(path, true)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
