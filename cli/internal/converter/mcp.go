package converter

import (
	"encoding/json"
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
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

	// OpenCode-specific (preserved in canonical for round-trips)
	Environment  map[string]string `json:"environment,omitempty"`  // OpenCode uses "environment" not "env"
	CommandArray []string          `json:"commandArray,omitempty"` // OpenCode command as array
	Enabled      *bool             `json:"enabled,omitempty"`      // OpenCode uses "enabled" (true default) not "disabled"
	Timeout      int               `json:"timeout,omitempty"`      // OpenCode timeout in ms
	OAuth        json.RawMessage   `json:"oauth,omitempty"`        // OAuth config (OpenCode + Claude Code; preserved opaque)
}

// mcpConfig wraps one or more server configs.
type mcpConfig struct {
	MCPServers map[string]mcpServerConfig `json:"mcpServers"`
}

// rooCodeMCPServerConfig is Roo Code's per-server format.
// Roo Code uses the standard mcpServers key but only supports a core subset of fields.
type rooCodeMCPServerConfig struct {
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Disabled    bool              `json:"disabled,omitempty"`
	Type        string            `json:"type,omitempty"`
	URL         string            `json:"url,omitempty"`
	AlwaysAllow []string          `json:"alwaysAllow,omitempty"`
}

type rooCodeMCPConfig struct {
	MCPServers map[string]rooCodeMCPServerConfig `json:"mcpServers"`
}

// clineMCPServerConfig is Cline's per-server format.
// Cline uses the mcpServers key like Claude Code, but alwaysAllow instead of autoApprove.
type clineMCPServerConfig struct {
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	AlwaysAllow []string          `json:"alwaysAllow,omitempty"`
	Disabled    bool              `json:"disabled,omitempty"`
}

type clineMCPConfig struct {
	MCPServers map[string]clineMCPServerConfig `json:"mcpServers"`
}

// opencodeMCPConfig wraps OpenCode's mcp section format.
// OpenCode uses "mcp" (not "mcpServers") as the top-level key.
type opencodeMCPConfig struct {
	MCP map[string]opencodeServerConfig `json:"mcp"`
}

type opencodeServerConfig struct {
	Type        string            `json:"type,omitempty"`
	Command     []string          `json:"command,omitempty"` // array form
	Environment map[string]string `json:"environment,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Timeout     int               `json:"timeout,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	OAuth       json.RawMessage   `json:"oauth,omitempty"`
}

type zedContextServer struct {
	Source  string            `json:"source"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type zedContextServersConfig struct {
	ContextServers map[string]zedContextServer `json:"context_servers"`
}

type MCPConverter struct{}

func (c *MCPConverter) ContentType() catalog.ContentType {
	return catalog.MCP
}

func (c *MCPConverter) Canonicalize(content []byte, sourceProvider string) (*Result, error) {
	if sourceProvider == "opencode" {
		return canonicalizeOpencodeMCP(content)
	}
	if sourceProvider == "zed" {
		return canonicalizeZedMCP(content)
	}
	if sourceProvider == "cline" {
		return canonicalizeClineMCP(content)
	}
	if sourceProvider == "roo-code" {
		return canonicalizeRooCodeMCP(content)
	}

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

func canonicalizeOpencodeMCP(content []byte) (*Result, error) {
	var src opencodeMCPConfig
	if err := ParseJSONC(content, &src); err != nil {
		return nil, fmt.Errorf("parsing OpenCode MCP config: %w", err)
	}

	out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}

	for name, s := range src.MCP {
		canonical := mcpServerConfig{
			URL:     s.URL,
			Headers: s.Headers,
			Timeout: s.Timeout,
			OAuth:   s.OAuth,
		}

		// command array → command + args
		if len(s.Command) > 0 {
			canonical.Command = s.Command[0]
			if len(s.Command) > 1 {
				canonical.Args = s.Command[1:]
			}
		}

		// environment → env
		if len(s.Environment) > 0 {
			canonical.Env = s.Environment
		}

		// enabled → disabled (flip polarity)
		// OpenCode default is enabled=true, so only set disabled when explicitly false.
		if s.Enabled != nil && !*s.Enabled {
			canonical.Disabled = true
		}

		// "remote" type → "sse"
		switch s.Type {
		case "remote":
			canonical.Type = "sse"
		case "local":
			// local maps to stdio; leave Type empty (implicit default)
		default:
			canonical.Type = s.Type
		}

		out.MCPServers[name] = canonical
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json"}, nil
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
	case "opencode":
		return renderOpencodeMCP(cfg)
	case "zed":
		return renderZedMCP(cfg)
	case "cline":
		return renderClineMCP(cfg)
	case "roo-code":
		return renderRooCodeMCP(cfg)
	case "kiro":
		return renderKiroMCP(cfg)
	case "cursor":
		// Cursor uses .cursor/mcp.json with the same mcpServers key and transport
		// types as Claude Code. Route through the same renderer.
		return renderCursorMCP(cfg)
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
			OAuth:       server.OAuth,
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

// renderCursorMCP renders canonical MCP config to Cursor's .cursor/mcp.json format.
// Cursor uses the same mcpServers schema as Claude Code (command, args, env, url, etc.).
// Provider-specific fields from other providers (trust, includeTools, etc.) are dropped.
func renderCursorMCP(cfg mcpConfig) (*Result, error) {
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

		// Warn about dropped OAuth config
		if len(server.OAuth) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: oauth config may not be supported by Cursor", name))
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
		if len(server.OAuth) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: oauth config may not be supported by Gemini CLI", name))
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
		if len(server.OAuth) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: oauth config may not be supported by Copilot CLI", name))
		}

		out.MCPServers[name] = s
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json", Warnings: warnings}, nil
}

func renderOpencodeMCP(cfg mcpConfig) (*Result, error) {
	var warnings []string
	out := opencodeMCPConfig{MCP: make(map[string]opencodeServerConfig)}

	for name, server := range cfg.MCPServers {
		s := opencodeServerConfig{
			URL:     server.URL,
			Headers: server.Headers,
			Timeout: server.Timeout,
			OAuth:   server.OAuth,
		}

		// command + args → command array
		if server.Command != "" {
			s.Command = append([]string{server.Command}, server.Args...)
		}

		// env → environment
		if len(server.Env) > 0 {
			s.Environment = server.Env
		}

		// disabled → enabled (flip polarity)
		// Only emit enabled when the server is explicitly disabled (avoid cluttering output).
		if server.Disabled {
			f := false
			s.Enabled = &f
		}

		// Set type: "local" for stdio, "remote" for HTTP
		if server.URL != "" {
			s.Type = "remote"
		} else {
			s.Type = "local"
		}

		// Warn about dropped provider-specific fields
		if len(server.AutoApprove) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: autoApprove dropped (Claude-specific)", name))
		}
		if server.Trust != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
		}
		if len(server.IncludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: includeTools dropped (Gemini-specific)", name))
		}
		if len(server.ExcludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: excludeTools dropped (Gemini-specific)", name))
		}
		if server.Cwd != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: cwd dropped (not supported by OpenCode)", name))
		}

		out.MCP[name] = s
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "opencode.json", Warnings: warnings}, nil
}

func canonicalizeZedMCP(content []byte) (*Result, error) {
	var src zedContextServersConfig
	if err := json.Unmarshal(content, &src); err != nil {
		return nil, fmt.Errorf("parsing Zed MCP config: %w", err)
	}

	out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}

	for name, s := range src.ContextServers {
		// Drop "source" field — it's Zed-specific metadata, not meaningful in canonical form.
		out.MCPServers[name] = mcpServerConfig{
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
		}
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json"}, nil
}

func canonicalizeClineMCP(content []byte) (*Result, error) {
	var src clineMCPConfig
	if err := json.Unmarshal(content, &src); err != nil {
		return nil, fmt.Errorf("parsing Cline MCP config: %w", err)
	}

	out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}

	for name, s := range src.MCPServers {
		out.MCPServers[name] = mcpServerConfig{
			Command:     s.Command,
			Args:        s.Args,
			Env:         s.Env,
			AutoApprove: s.AlwaysAllow, // alwaysAllow → autoApprove in canonical
			Disabled:    s.Disabled,
		}
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json"}, nil
}

func canonicalizeRooCodeMCP(content []byte) (*Result, error) {
	var src rooCodeMCPConfig
	if err := json.Unmarshal(content, &src); err != nil {
		return nil, fmt.Errorf("parsing Roo Code MCP config: %w", err)
	}

	out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}

	for name, s := range src.MCPServers {
		out.MCPServers[name] = mcpServerConfig{
			Command:     s.Command,
			Args:        s.Args,
			Env:         s.Env,
			Disabled:    s.Disabled,
			Type:        s.Type,
			URL:         s.URL,
			AutoApprove: s.AlwaysAllow, // alwaysAllow → autoApprove in canonical
		}
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json"}, nil
}

func renderClineMCP(cfg mcpConfig) (*Result, error) {
	var warnings []string
	out := clineMCPConfig{MCPServers: make(map[string]clineMCPServerConfig)}

	for name, server := range cfg.MCPServers {
		// Cline only supports stdio; warn and skip HTTP servers.
		if server.URL != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: skipped (Cline only supports stdio, not HTTP)", name))
			continue
		}

		out.MCPServers[name] = clineMCPServerConfig{
			Command:     server.Command,
			Args:        server.Args,
			Env:         server.Env,
			AlwaysAllow: server.AutoApprove, // autoApprove → alwaysAllow
			Disabled:    server.Disabled,
		}

		// Warn about dropped provider-specific fields
		if server.Trust != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
		}
		if len(server.IncludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: includeTools dropped (Gemini-specific)", name))
		}
		if len(server.ExcludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: excludeTools dropped (Gemini-specific)", name))
		}
		if server.Cwd != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: cwd dropped (not supported by Cline)", name))
		}
		if len(server.OAuth) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: oauth config may not be supported by Cline", name))
		}
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "cline_mcp_settings.json", Warnings: warnings}, nil
}

func renderRooCodeMCP(cfg mcpConfig) (*Result, error) {
	var warnings []string
	out := rooCodeMCPConfig{MCPServers: make(map[string]rooCodeMCPServerConfig)}

	for name, server := range cfg.MCPServers {
		out.MCPServers[name] = rooCodeMCPServerConfig{
			Command:     server.Command,
			Args:        server.Args,
			Env:         server.Env,
			Disabled:    server.Disabled,
			Type:        server.Type,
			URL:         server.URL,
			AlwaysAllow: server.AutoApprove, // autoApprove → alwaysAllow
		}

		// Warn about dropped provider-specific fields
		if server.Cwd != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: cwd dropped (not supported by Roo Code)", name))
		}
		if len(server.Headers) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: headers dropped (not supported by Roo Code)", name))
		}
		if server.Trust != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
		}
		if len(server.IncludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: includeTools dropped (Gemini-specific)", name))
		}
		if len(server.ExcludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: excludeTools dropped (Gemini-specific)", name))
		}
		if len(server.OAuth) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: oauth config may not be supported by Roo Code", name))
		}
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json", Warnings: warnings}, nil
}

// kiroServerConfig is Kiro's on-disk MCP server format.
// Similar to canonical but adds disabledTools.
type kiroServerConfig struct {
	Command       string            `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	URL           string            `json:"url,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Disabled      bool              `json:"disabled,omitempty"`
	AutoApprove   []string          `json:"autoApprove,omitempty"`
	DisabledTools []string          `json:"disabledTools,omitempty"`
}

type kiroMCPConfig struct {
	MCPServers map[string]kiroServerConfig `json:"mcpServers"`
}

func renderKiroMCP(cfg mcpConfig) (*Result, error) {
	var warnings []string
	kc := kiroMCPConfig{MCPServers: make(map[string]kiroServerConfig)}

	for name, server := range cfg.MCPServers {
		s := kiroServerConfig{
			Command:     server.Command,
			Args:        server.Args,
			Env:         server.Env,
			URL:         server.URL,
			Headers:     server.Headers,
			Disabled:    server.Disabled,
			AutoApprove: server.AutoApprove,
		}

		// Warn about dropped Gemini-specific fields
		if server.Trust != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
		}
		if len(server.IncludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: includeTools dropped (Gemini-specific)", name))
		}
		if len(server.OAuth) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: oauth config may not be supported by Kiro", name))
		}

		kc.MCPServers[name] = s
	}

	result, err := json.MarshalIndent(kc, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "mcp.json", Warnings: warnings}, nil
}

func renderZedMCP(cfg mcpConfig) (*Result, error) {
	var warnings []string
	out := zedContextServersConfig{ContextServers: make(map[string]zedContextServer)}

	for name, server := range cfg.MCPServers {
		// Zed only supports stdio; skip HTTP servers.
		if server.URL != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: skipped (Zed only supports stdio, not HTTP)", name))
			continue
		}

		out.ContextServers[name] = zedContextServer{
			Source:  "custom",
			Command: server.Command,
			Args:    server.Args,
			Env:     server.Env,
		}

		// Warn about dropped provider-specific fields
		if server.Cwd != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: cwd dropped (not supported by Zed)", name))
		}
		if len(server.AutoApprove) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: autoApprove dropped (Claude-specific)", name))
		}
		if server.Trust != "" {
			warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
		}
		if len(server.IncludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: includeTools dropped (Gemini-specific)", name))
		}
		if len(server.ExcludeTools) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: excludeTools dropped (Gemini-specific)", name))
		}
		if len(server.OAuth) > 0 {
			warnings = append(warnings, fmt.Sprintf("server %q: oauth config may not be supported by Zed", name))
		}
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "settings.json", Warnings: warnings}, nil
}
