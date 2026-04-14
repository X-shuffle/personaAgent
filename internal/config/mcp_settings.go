package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	defaultMCPSettingsPath = "mcp_settings.json"
	defaultMCPTimeoutMS    = 30000
)

type MCPServerConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	Cwd       string            `json:"cwd"`
	Transport string            `json:"transport"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Timeout   int               `json:"timeout"`
	Disabled  bool              `json:"disabled"`
}

type mcpSettingsFile struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

func loadMCPSettings(path string, required bool) (map[string]MCPServerConfig, map[string]MCPServerConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultMCPSettingsPath
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !required {
			return map[string]MCPServerConfig{}, map[string]MCPServerConfig{}, nil
		}
		return nil, nil, fmt.Errorf("read mcp settings %q: %w", path, err)
	}

	var raw mcpSettingsFile
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("parse mcp settings %q: %w", path, err)
	}

	all := make(map[string]MCPServerConfig, len(raw.MCPServers))
	active := make(map[string]MCPServerConfig, len(raw.MCPServers))
	for name, server := range raw.MCPServers {
		normalizedName := strings.TrimSpace(name)
		if normalizedName == "" {
			return nil, nil, fmt.Errorf("parse mcp settings %q: empty server name", path)
		}

		normalized, err := normalizeAndValidateMCPServer(normalizedName, server)
		if err != nil {
			return nil, nil, fmt.Errorf("parse mcp settings %q: %w", path, err)
		}

		all[normalizedName] = normalized
		if !normalized.Disabled {
			active[normalizedName] = normalized
		}
	}

	return all, active, nil
}

func normalizeAndValidateMCPServer(name string, server MCPServerConfig) (MCPServerConfig, error) {
	server.Command = strings.TrimSpace(server.Command)
	server.Cwd = strings.TrimSpace(server.Cwd)
	server.Transport = strings.ToLower(strings.TrimSpace(server.Transport))
	server.URL = strings.TrimSpace(server.URL)

	for i := range server.Args {
		server.Args[i] = strings.TrimSpace(server.Args[i])
	}

	server.Env = normalizeStringMap(server.Env)
	server.Headers = normalizeStringMap(server.Headers)

	if server.Timeout <= 0 {
		server.Timeout = defaultMCPTimeoutMS
	}

	switch server.Transport {
	case "stdio":
		if server.Command == "" {
			return MCPServerConfig{}, fmt.Errorf("server %q: command is required for stdio transport", name)
		}
		if server.URL != "" {
			return MCPServerConfig{}, fmt.Errorf("server %q: url must be empty for stdio transport", name)
		}
	case "sse", "http":
		if server.URL == "" {
			return MCPServerConfig{}, fmt.Errorf("server %q: url is required for %s transport", name, server.Transport)
		}
	default:
		return MCPServerConfig{}, fmt.Errorf("server %q: unsupported transport %q", name, server.Transport)
	}

	return server, nil
}

func normalizeStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		tk := strings.TrimSpace(k)
		if tk == "" {
			continue
		}
		out[tk] = strings.TrimSpace(v)
	}
	return out
}
