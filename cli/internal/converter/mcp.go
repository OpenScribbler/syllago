package converter

import (
	"encoding/json"
	"fmt"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
)

func init() {
	Register(&MCPConverter{})
}

// mcpServerConfig is the canonical MCP server configuration (superset of all providers).
type mcpServerConfig struct {
	// Universal fields
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`

	// HTTP transport
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Type    string            `json:"type,omitempty"` // "stdio" | "sse" | "streamable-http"

	// Provider-specific (preserved in canonical for round-trips)
	Trust        string   `json:"trust,omitempty"`        // Gemini-specific
	IncludeTools []string `json:"includeTools,omitempty"` // Gemini-specific
	ExcludeTools []string `json:"excludeTools,omitempty"` // Gemini-specific
	Disabled     bool     `json:"disabled,omitempty"`     // Runtime state
	AutoApprove  []string `json:"autoApprove,omitempty"`  // Claude-specific

	// Gemini alternate field names
	HTTPUrl string `json:"httpUrl,omitempty"` // Gemini uses httpUrl instead of url
}

// mcpConfig wraps one or more server configs.
type mcpConfig struct {
	MCPServers map[string]mcpServerConfig `json:"mcpServers"`
}

type MCPConverter struct{}

func (c *MCPConverter) ContentType() catalog.ContentType {
	return catalog.MCP
}

func (c *MCPConverter) Canonicalize(content []byte, sourceProvider string) (*Result, error) {
	var cfg mcpConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parsing MCP config: %w", err)
	}

	// Normalize: merge httpUrl into url field
	for name, server := range cfg.MCPServers {
		if server.HTTPUrl != "" && server.URL == "" {
			server.URL = server.HTTPUrl
			server.HTTPUrl = ""
			if server.Type == "" {
				server.Type = "sse" // Infer transport type from httpUrl
			}
		}
		cfg.MCPServers[name] = server
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: out, Filename: "mcp.json"}, nil
}

func (c *MCPConverter) Render(content []byte, target provider.Provider) (*Result, error) {
	var cfg mcpConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parsing canonical MCP config: %w", err)
	}

	switch target.Slug {
	case "gemini-cli":
		return renderGeminiMCP(cfg)
	case "copilot-cli":
		return renderCopilotMCP(cfg)
	default:
		// Claude Code — emit with Claude-compatible fields only
		return renderClaudeMCP(cfg)
	}
}

// --- Renderers ---

func renderClaudeMCP(cfg mcpConfig) (*Result, error) {
	var warnings []string
	out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}

	for name, server := range cfg.MCPServers {
		s := mcpServerConfig{
			Command:     server.Command,
			Args:        server.Args,
			Env:         server.Env,
			Cwd:         server.Cwd,
			URL:         server.URL,
			Headers:     server.Headers,
			Type:        server.Type,
			AutoApprove: server.AutoApprove,
		}

		// Warn about dropped Gemini-specific fields
		if server.Trust != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: trust field dropped (Gemini-specific)", name))
		}
		if len(server.IncludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: includeTools dropped (Gemini-specific)", name))
		}
		if len(server.ExcludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: excludeTools dropped (Gemini-specific)", name))
		}

		out.MCPServers[name] = s
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json", Warnings: warnings}, nil
}

func renderGeminiMCP(cfg mcpConfig) (*Result, error) {
	var warnings []string
	out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}

	for name, server := range cfg.MCPServers {
		s := mcpServerConfig{
			Command:      server.Command,
			Args:         server.Args,
			Env:          server.Env,
			Cwd:          server.Cwd,
			Headers:      server.Headers,
			Trust:        server.Trust,
			IncludeTools: server.IncludeTools,
			ExcludeTools: server.ExcludeTools,
		}

		// Gemini uses httpUrl for HTTP transport
		if server.URL != "" {
			s.HTTPUrl = server.URL
		}

		// Warn about dropped Claude-specific fields
		if len(server.AutoApprove) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: autoApprove dropped (Claude-specific)", name))
		}
		if server.Disabled {
			warnings = append(warnings, fmt.Sprintf("server %q: disabled state dropped (runtime-only)", name))
		}

		out.MCPServers[name] = s
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json", Warnings: warnings}, nil
}

func renderCopilotMCP(cfg mcpConfig) (*Result, error) {
	var warnings []string
	out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}

	for name, server := range cfg.MCPServers {
		s := mcpServerConfig{
			Command: server.Command,
			Args:    server.Args,
			Env:     server.Env,
			Cwd:     server.Cwd,
			URL:     server.URL,
			Headers: server.Headers,
			Type:    server.Type,
		}

		// Warn about all provider-specific fields
		if server.Trust != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
		}
		if len(server.IncludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: includeTools dropped (Gemini-specific)", name))
		}
		if len(server.ExcludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: excludeTools dropped (Gemini-specific)", name))
		}
		if len(server.AutoApprove) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: autoApprove dropped (Claude-specific)", name))
		}

		out.MCPServers[name] = s
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json", Warnings: warnings}, nil
}
