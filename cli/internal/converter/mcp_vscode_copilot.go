package converter

import (
	"encoding/json"
	"fmt"
)

// vscodeMCPInput represents a VS Code input variable (promptString, etc.).
type vscodeMCPInput struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Password    bool   `json:"password,omitempty"`
}

// vscodeMCPServerConfig is VS Code Copilot's per-server format.
// VS Code uses "servers" (not "mcpServers") as the top-level key and has
// provider-specific fields like envFile and sandbox.
type vscodeMCPServerConfig struct {
	Type           string            `json:"type,omitempty"` // stdio, http, sse
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	URL            string            `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	EnvFile        string            `json:"envFile,omitempty"`
	SandboxEnabled *bool             `json:"sandboxEnabled,omitempty"`
	Sandbox        json.RawMessage   `json:"sandbox,omitempty"` // Preserve opaque
}

type vscodeMCPConfig struct {
	Inputs  []vscodeMCPInput                 `json:"inputs,omitempty"`
	Servers map[string]vscodeMCPServerConfig `json:"servers"`
}

func canonicalizeVSCodeCopilotMCP(content []byte) (*Result, error) {
	var src vscodeMCPConfig
	if err := json.Unmarshal(content, &src); err != nil {
		return nil, fmt.Errorf("parsing VS Code MCP config: %w", err)
	}

	out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}

	for name, s := range src.Servers {
		canonical := mcpServerConfig{
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
			URL:     s.URL,
			Headers: s.Headers,
		}

		// Normalize transport type
		switch s.Type {
		case "http":
			canonical.Type = "streamable-http"
		case "sse":
			canonical.Type = "sse"
		default:
			// stdio or empty — leave as default
		}

		out.MCPServers[name] = canonical
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}

	var warnings []string
	if len(src.Inputs) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d input variable(s) dropped (VS Code-specific; env vars may contain ${input:...} references that won't resolve)", len(src.Inputs)))
	}
	for name, s := range src.Servers {
		if s.EnvFile != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: envFile dropped (VS Code-specific)", name))
		}
		if s.SandboxEnabled != nil {
			warnings = append(warnings, fmt.Sprintf("server %q: sandboxEnabled dropped (VS Code-specific)", name))
		}
		if len(s.Sandbox) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: sandbox config dropped (VS Code-specific)", name))
		}
	}

	return &Result{Content: result, Filename: "mcp.json", Warnings: warnings}, nil
}

func renderVSCodeCopilotMCP(cfg mcpConfig) (*Result, error) {
	var warnings []string
	out := vscodeMCPConfig{Servers: make(map[string]vscodeMCPServerConfig)}

	for name, server := range cfg.MCPServers {
		s := vscodeMCPServerConfig{
			Command: server.Command,
			Args:    server.Args,
			Env:     server.Env,
			URL:     server.URL,
			Headers: server.Headers,
		}

		// Set transport type
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

		// Warn about dropped provider-specific fields
		if server.Cwd != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: cwd dropped (not supported by VS Code Copilot)", name))
		}
		if len(server.AutoApprove) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: autoApprove dropped (Claude-specific)", name))
		}
		if server.Trust != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
		}
		if len(server.IncludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: includeTools dropped (Gemini/Amp-specific)", name))
		}
		if len(server.ExcludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: excludeTools dropped (Gemini-specific)", name))
		}
		if len(server.DisabledTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: disabledTools dropped (not supported by VS Code Copilot)", name))
		}
		if len(server.OAuth) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: oauth config may not be supported by VS Code Copilot", name))
		}

		out.Servers[name] = s
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json", Warnings: warnings}, nil
}
