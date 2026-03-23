package converter

import (
	"encoding/json"
	"fmt"

	toml "github.com/pelletier/go-toml/v2"
)

// codexMCPServerConfig is Codex CLI's per-server format (TOML).
// Codex uses snake_case field names and unique fields like bearer_token_env_var.
type codexMCPServerConfig struct {
	Type              string            `toml:"type,omitempty"`
	Command           string            `toml:"command,omitempty"`
	Args              []string          `toml:"args,omitempty"`
	EnvVars           map[string]string `toml:"env_vars,omitempty"`
	URL               string            `toml:"url,omitempty"`
	BearerTokenEnvVar string            `toml:"bearer_token_env_var,omitempty"`
	EnvHTTPHeaders    map[string]string `toml:"env_http_headers,omitempty"`
	Enabled           *bool             `toml:"enabled,omitempty"`
	EnabledTools      []string          `toml:"enabled_tools,omitempty"`
	DisabledTools     []string          `toml:"disabled_tools,omitempty"`
}

type codexMCPConfig struct {
	MCPServers map[string]codexMCPServerConfig `toml:"mcp_servers"`
}

func canonicalizeCodexMCP(content []byte) (*Result, error) {
	var src codexMCPConfig
	if err := toml.Unmarshal(content, &src); err != nil {
		return nil, fmt.Errorf("parsing Codex TOML MCP config: %w", err)
	}

	out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}

	for name, s := range src.MCPServers {
		canonical := mcpServerConfig{
			Command: s.Command,
			Args:    s.Args,
			URL:     s.URL,
		}

		// env_vars -> env
		if len(s.EnvVars) > 0 {
			canonical.Env = s.EnvVars
		}

		// bearer_token_env_var -> Authorization header with env var reference
		if s.BearerTokenEnvVar != "" {
			if canonical.Headers == nil {
				canonical.Headers = make(map[string]string)
			}
			canonical.Headers["Authorization"] = "Bearer ${" + s.BearerTokenEnvVar + "}"
		}

		// env_http_headers -> headers
		if len(s.EnvHTTPHeaders) > 0 {
			if canonical.Headers == nil {
				canonical.Headers = make(map[string]string)
			}
			for k, v := range s.EnvHTTPHeaders {
				canonical.Headers[k] = v
			}
		}

		// enabled -> disabled (flip polarity; Codex default is enabled=true)
		if s.Enabled != nil && !*s.Enabled {
			canonical.Disabled = true
		}

		// enabled_tools -> includeTools
		if len(s.EnabledTools) > 0 {
			canonical.IncludeTools = s.EnabledTools
		}

		// disabled_tools -> excludeTools
		if len(s.DisabledTools) > 0 {
			canonical.ExcludeTools = s.DisabledTools
		}

		// Normalize transport type
		switch s.Type {
		case "http":
			canonical.Type = "streamable-http"
		case "sse":
			canonical.Type = "sse"
		default:
			// stdio -- leave empty (implicit default)
		}

		out.MCPServers[name] = canonical
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json"}, nil
}

func renderCodexMCP(cfg mcpConfig) (*Result, error) {
	var warnings []string
	out := codexMCPConfig{MCPServers: make(map[string]codexMCPServerConfig)}

	for name, server := range cfg.MCPServers {
		s := codexMCPServerConfig{
			Command: server.Command,
			Args:    server.Args,
			URL:     server.URL,
		}

		// env -> env_vars
		if len(server.Env) > 0 {
			s.EnvVars = server.Env
		}

		// headers -> env_http_headers
		if len(server.Headers) > 0 {
			s.EnvHTTPHeaders = server.Headers
		}

		// disabled -> enabled (flip polarity)
		if server.Disabled {
			f := false
			s.Enabled = &f
		}

		// includeTools -> enabled_tools
		if len(server.IncludeTools) > 0 {
			s.EnabledTools = server.IncludeTools
		}

		// excludeTools -> disabled_tools
		if len(server.ExcludeTools) > 0 {
			s.DisabledTools = server.ExcludeTools
		}

		// Set type
		switch server.Type {
		case "streamable-http":
			s.Type = "http"
		case "sse":
			s.Type = "sse"
		default:
			if server.Command != "" {
				s.Type = "stdio"
			}
		}

		// Warn about dropped fields
		if server.Cwd != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: cwd dropped (not supported by Codex)", name))
		}
		if len(server.AutoApprove) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: autoApprove dropped (Claude-specific)", name))
		}
		if server.Trust != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
		}
		if len(server.DisabledTools) > 0 && len(server.ExcludeTools) == 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: disabledTools dropped (not supported by Codex)", name))
		}
		if len(server.OAuth) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: oauth config may not be supported by Codex", name))
		}

		out.MCPServers[name] = s
	}

	result, err := toml.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "codex.toml", Warnings: warnings}, nil
}
