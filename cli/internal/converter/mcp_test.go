package converter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestClaudeMCPToGemini(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"},
				"autoApprove": ["search_repositories"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "npx")
	assertContains(t, out, "@modelcontextprotocol/server-github")
	assertContains(t, out, "GITHUB_TOKEN")
	assertNotContains(t, out, "autoApprove")

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about dropped autoApprove")
	}
}

func TestGeminiMCPToClaude(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"myserver": {
				"httpUrl": "https://api.example.com/mcp",
				"trust": "trusted",
				"includeTools": ["search", "read"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// httpUrl should be normalized to url
	assertContains(t, out, "\"url\"")
	assertNotContains(t, out, "httpUrl")
	assertNotContains(t, out, "trust")
	assertNotContains(t, out, "includeTools")

	if len(result.Warnings) < 2 {
		t.Fatalf("expected at least 2 warnings, got %d", len(result.Warnings))
	}
}

func TestMCPHttpUrlNormalization(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"myapi": {
				"httpUrl": "https://api.example.com/sse"
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(canonical.Content, &cfg)

	server := cfg.MCPServers["myapi"]
	assertEqual(t, "https://api.example.com/sse", server.URL)
	assertEqual(t, "", server.HTTPUrl)
	assertEqual(t, "sse", server.Type)
}

func TestMCPStdioPreserved(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"local": {
				"command": "node",
				"args": ["server.js"],
				"env": {"PORT": "3000"},
				"cwd": "/app"
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "node")
	assertContains(t, out, "server.js")
	assertContains(t, out, "3000")
	assertContains(t, out, "/app")

	if len(result.Warnings) > 0 {
		t.Fatalf("expected no warnings for stdio server, got: %v", result.Warnings)
	}
}

func TestMCPToCopilot(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"test": {
				"command": "python",
				"args": ["server.py"],
				"trust": "high",
				"autoApprove": ["read"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "python")
	assertNotContains(t, out, "trust")
	assertNotContains(t, out, "autoApprove")

	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings about dropped fields")
	}
}

func TestZedMCPRender(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"mytool": {
				"command": "node",
				"args": ["server.js", "--port", "3000"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Zed)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "context_servers")
	assertNotContains(t, out, "mcpServers")
	assertContains(t, out, `"source": "custom"`)
	assertContains(t, out, "node")
	assertContains(t, out, "server.js")
	assertEqual(t, "settings.json", result.Filename)

	if len(result.Warnings) > 0 {
		t.Fatalf("expected no warnings for stdio server, got: %v", result.Warnings)
	}
}

func TestZedMCPHTTPServerRendered(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"remoteapi": {
				"url": "https://api.example.com/mcp",
				"type": "sse",
				"headers": {"Authorization": "Bearer tok"}
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Zed)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "context_servers")
	assertContains(t, out, "remoteapi")
	assertContains(t, out, "https://api.example.com/mcp")
	assertContains(t, out, `"source": "custom"`)
	assertContains(t, out, "Authorization")
	assertContains(t, out, "Bearer tok")
	// URL-based servers should not have command/args
	assertNotContains(t, out, "command")

	if len(result.Warnings) > 0 {
		t.Fatalf("expected no warnings for URL server, got: %v", result.Warnings)
	}
}

func TestZedMCPCanonicalize(t *testing.T) {
	input := []byte(`{
		"context_servers": {
			"mytool": {
				"source": "custom",
				"command": "npx",
				"args": ["-y", "some-mcp-server"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "zed")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "mcpServers")
	assertNotContains(t, out, "source")
	assertContains(t, out, "npx")
	assertContains(t, out, "some-mcp-server")
}

func TestZedMCPCanonicalizeURL(t *testing.T) {
	input := []byte(`{
		"context_servers": {
			"remote": {
				"source": "custom",
				"url": "https://api.example.com/mcp",
				"headers": {"Authorization": "Bearer tok"}
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "zed")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "mcpServers")
	assertNotContains(t, out, "source")
	assertContains(t, out, "https://api.example.com/mcp")
	assertContains(t, out, "Authorization")
	assertContains(t, out, "Bearer tok")
	assertContains(t, out, `"type": "sse"`)
}

func TestClineMCPRender(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"},
				"autoApprove": ["search_repositories"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cline)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "alwaysAllow")
	assertNotContains(t, out, "autoApprove")
	assertContains(t, out, "search_repositories")
	assertContains(t, out, "npx")
	assertContains(t, out, "GITHUB_TOKEN")
	assertNotContains(t, out, "cwd")
	assertNotContains(t, out, "trust")
	assertNotContains(t, out, "includeTools")
	assertEqual(t, "cline_mcp_settings.json", result.Filename)
}

func TestClineMCPPreservesHTTPServers(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"remoteapi": {
				"url": "https://api.example.com/mcp",
				"type": "sse"
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cline)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Cline supports SSE transport — server should be emitted, not skipped
	out := string(result.Content)
	assertContains(t, out, "remoteapi")
	assertContains(t, out, "api.example.com")
}

func TestClineMCPCanonicalizeSSE(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"url": "https://sse.example.com/events",
				"headers": {"Authorization": "Bearer tok123"}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "cline")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["remote"]
	assertEqual(t, "https://sse.example.com/events", server.URL)
	assertEqual(t, "sse", server.Type)
	assertEqual(t, "Bearer tok123", server.Headers["Authorization"])
}

func TestClineMCPSSERoundTrip(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"url": "https://sse.example.com/events",
				"headers": {"Authorization": "Bearer tok123"},
				"alwaysAllow": ["read"]
			}
		}
	}`)

	conv := &MCPConverter{}

	// Cline → canonical
	canonical, err := conv.Canonicalize(input, "cline")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical → Cline
	result, err := conv.Render(canonical.Content, provider.Cline)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "sse.example.com")
	assertContains(t, out, "Authorization")
	assertContains(t, out, "Bearer tok123")
	assertContains(t, out, "alwaysAllow")
	assertContains(t, out, "read")
	assertNotContains(t, out, "autoApprove")
}

func TestClineMCPCanonicalize(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"mytool": {
				"command": "node",
				"args": ["server.js"],
				"alwaysAllow": ["read_file", "write_file"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "cline")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "autoApprove")
	assertNotContains(t, out, "alwaysAllow")
	assertContains(t, out, "read_file")
	assertContains(t, out, "write_file")
	assertContains(t, out, "mcpServers")
}

// --- Roo Code MCP ---

func TestRooCodeMCPRender(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"},
				"autoApprove": ["search_repositories"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "npx")
	assertContains(t, out, "GITHUB_TOKEN")
	assertNotContains(t, out, "autoApprove")
	assertContains(t, out, "alwaysAllow")
	assertContains(t, out, "search_repositories")
	assertEqual(t, "mcp.json", result.Filename)

	// No warning about autoApprove — Roo Code supports it as alwaysAllow
	for _, w := range result.Warnings {
		if strings.Contains(w, "autoApprove") {
			t.Errorf("unexpected autoApprove warning: %s", w)
		}
	}
}

func TestRooCodeMCPCanonicalizeAlwaysAllow(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"},
				"alwaysAllow": ["search_repositories", "list_issues"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "roo-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	// alwaysAllow should be mapped to autoApprove in canonical form
	assertContains(t, out, "autoApprove")
	assertContains(t, out, "search_repositories")
	assertContains(t, out, "list_issues")
	assertNotContains(t, out, "alwaysAllow")
}

func TestRooCodeMCPAlwaysAllowRoundTrip(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"alwaysAllow": ["search_repositories"]
			}
		}
	}`)

	conv := &MCPConverter{}

	// Roo Code → canonical
	canonical, err := conv.Canonicalize(input, "roo-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical → Roo Code
	result, err := conv.Render(canonical.Content, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "alwaysAllow")
	assertContains(t, out, "search_repositories")
	assertNotContains(t, out, "autoApprove")
}

func TestRooCodeMCPPreservesHTTPServers(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"remoteapi": {
				"url": "https://api.example.com/mcp",
				"type": "sse"
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Roo Code supports HTTP servers (unlike Zed/Cline)
	out := string(result.Content)
	assertContains(t, out, "api.example.com")
	assertContains(t, out, "sse")
}

// --- OpenCode MCP ---

func TestOpenCodeMCPRender(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"filesystem": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-filesystem", "/home"],
				"env": {"DEBUG": "1"}
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// OpenCode uses "mcp" key, not "mcpServers"
	assertContains(t, out, `"mcp"`)
	assertNotContains(t, out, `"mcpServers"`)
	// Command must be an array
	assertContains(t, out, `"command": [`)
	assertContains(t, out, `"npx"`)
	// Env key must be "environment"
	assertContains(t, out, `"environment"`)
	assertNotContains(t, out, `"env"`)
	// Type must be "local" for stdio
	assertContains(t, out, `"type": "local"`)
	assertEqual(t, "opencode.json", result.Filename)
}

func TestOpenCodeMCPCanonicalize(t *testing.T) {
	input := []byte(`{
		"mcp": {
			"local-server": {
				"type": "local",
				"command": ["npx", "-y", "my-mcp"],
				"environment": {"API_KEY": "secret"},
				"enabled": true,
				"timeout": 5000
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, `"command": "npx"`)
	assertContains(t, out, `"args"`)
	assertContains(t, out, `"env"`)
	assertContains(t, out, "API_KEY")
	assertNotContains(t, out, "environment")
}

func TestOpenCodeMCPCanonicalizeJSONC(t *testing.T) {
	input := []byte(`{
		// Main MCP config for OpenCode
		"mcp": {
			/* database server */
			"db": {
				"type": "local",
				"command": ["db-mcp"],
				"enabled": false
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize with JSONC comments: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "db")
	// enabled: false should map to disabled: true
	assertContains(t, out, `"disabled": true`)
}

func TestOpenCodeMCPRemoteServer(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"url": "https://mcp.example.com",
				"type": "sse"
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, `"type": "remote"`)
	assertContains(t, out, "mcp.example.com")
}

// --- Kiro MCP ---

func TestKiroMCPRender(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"},
				"autoApprove": ["search_repositories"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "npx")
	assertContains(t, out, "GITHUB_TOKEN")
	// Kiro preserves autoApprove
	assertContains(t, out, "autoApprove")
	assertContains(t, out, "search_repositories")
	assertEqual(t, "mcp.json", result.Filename)
}

func TestKiroMCPDropsGeminiFields(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"server": {
				"command": "node",
				"args": ["s.js"],
				"trust": "high",
				"includeTools": ["search"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings about dropped Gemini fields")
	}
}

func TestRooCodeMCPPreservesCwdHeadersTimeout(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"local": {
				"command": "node",
				"args": ["server.js"],
				"cwd": "/app",
				"headers": {"Authorization": "Bearer tok"},
				"timeout": 30
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, `"cwd"`)
	assertContains(t, out, "/app")
	assertContains(t, out, `"headers"`)
	assertContains(t, out, "Bearer tok")
	assertContains(t, out, `"timeout"`)
	assertContains(t, out, "node")

	// No warnings about cwd, headers, or timeout — Roo Code supports them
	for _, w := range result.Warnings {
		if strings.Contains(w, "cwd") {
			t.Errorf("unexpected cwd warning: %s", w)
		}
		if strings.Contains(w, "headers") {
			t.Errorf("unexpected headers warning: %s", w)
		}
		if strings.Contains(w, "timeout") {
			t.Errorf("unexpected timeout warning: %s", w)
		}
	}
}

// --- Cursor MCP ---

func TestCursorMCPRender(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"},
				"autoApprove": ["search_repositories"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "npx")
	assertContains(t, out, "GITHUB_TOKEN")
	// autoApprove is not documented by Cursor — should be dropped with warning
	assertNotContains(t, out, "autoApprove")
	assertNotContains(t, out, "search_repositories")
	assertEqual(t, "mcp.json", result.Filename)

	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "autoApprove dropped") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected warning about autoApprove being dropped for Cursor")
	}
}

func TestCursorMCPCanonicalize(t *testing.T) {
	// Cursor uses .cursor/mcp.json with mcpServers key — same as Claude Code
	input := []byte(`{
		"mcpServers": {
			"local": {
				"command": "node",
				"args": ["server.js"],
				"env": {"PORT": "3000"}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "node")
	assertContains(t, out, "server.js")
	assertContains(t, out, "PORT")
}

func TestCursorMCPRoundTrip(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"},
				"autoApprove": ["search_repositories"]
			}
		}
	}`)

	conv := &MCPConverter{}

	// Cursor → canonical
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical → Cursor
	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "npx")
	// autoApprove should be dropped on Cursor render (not documented by Cursor)
	assertNotContains(t, out, "autoApprove")
}

func TestCursorMCPDropsGeminiFields(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"server": {
				"command": "node",
				"args": ["s.js"],
				"trust": "high",
				"includeTools": ["search"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertNotContains(t, out, "trust")
	assertNotContains(t, out, "includeTools")

	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings about dropped Gemini fields")
	}
}

// --- Windsurf MCP ---

func TestWindsurfMCPRender(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"},
				"autoApprove": ["search_repositories"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "npx")
	assertContains(t, out, "GITHUB_TOKEN")
	assertNotContains(t, out, "autoApprove")
	assertEqual(t, "mcp_config.json", result.Filename)

	// autoApprove should produce a warning
	hasAutoApproveWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "autoApprove") {
			hasAutoApproveWarning = true
			break
		}
	}
	if !hasAutoApproveWarning {
		t.Error("expected warning about dropped autoApprove")
	}
}

func TestWindsurfMCPCanonicalize(t *testing.T) {
	// Windsurf uses mcp_config.json with mcpServers key
	input := []byte(`{
		"mcpServers": {
			"local": {
				"command": "node",
				"args": ["server.js"],
				"env": {"PORT": "3000"}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "node")
	assertContains(t, out, "server.js")
	assertContains(t, out, "PORT")
}

func TestWindsurfMCPServerUrlNormalization(t *testing.T) {
	// Windsurf uses serverUrl for HTTP transport
	input := []byte(`{
		"mcpServers": {
			"remote-api": {
				"serverUrl": "https://api.example.com/mcp",
				"headers": {"Authorization": "Bearer token123"}
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// serverUrl should be normalized to url in canonical form
	out := string(canonical.Content)
	assertContains(t, out, `"url"`)
	assertContains(t, out, "api.example.com")
	assertNotContains(t, out, "serverUrl")
}

func TestWindsurfMCPRoundTrip(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"}
			}
		}
	}`)

	conv := &MCPConverter{}

	// Windsurf -> canonical
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical -> Windsurf
	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "npx")
	assertContains(t, out, "GITHUB_TOKEN")
	assertEqual(t, "mcp_config.json", result.Filename)
}

func TestWindsurfMCPServerUrlRoundTrip(t *testing.T) {
	// HTTP server with serverUrl should round-trip through canonical
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"serverUrl": "https://api.example.com/mcp",
				"headers": {"Authorization": "Bearer tok"}
			}
		}
	}`)

	conv := &MCPConverter{}

	// Windsurf -> canonical
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical -> Windsurf
	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "serverUrl")
	assertContains(t, out, "api.example.com")
	assertContains(t, out, "Authorization")
}

func TestWindsurfMCPSSEUrl(t *testing.T) {
	// SSE server with url field
	input := []byte(`{
		"mcpServers": {
			"sse-server": {
				"url": "https://sse.example.com/events"
			}
		}
	}`)

	conv := &MCPConverter{}

	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// SSE servers should use url, not serverUrl
	assertContains(t, out, `"url"`)
	assertContains(t, out, "sse.example.com")
}

func TestWindsurfMCPDropsGeminiFields(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"server": {
				"command": "node",
				"args": ["s.js"],
				"trust": "high",
				"includeTools": ["search"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertNotContains(t, out, "trust")
	assertNotContains(t, out, "includeTools")

	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings about dropped Gemini fields")
	}
}

// --- OAuth MCP Tests ---

func TestClaudeMCPOAuthRoundTrip(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"auth-server": {
				"url": "https://api.example.com/mcp",
				"type": "sse",
				"oauth": {
					"client_id": "my-client",
					"scopes": ["read", "write"],
					"auth_url": "https://auth.example.com/authorize"
				}
			}
		}
	}`)

	conv := &MCPConverter{}

	// Claude Code → canonical
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Verify OAuth preserved in canonical form
	out := string(canonical.Content)
	assertContains(t, out, "oauth")
	assertContains(t, out, "my-client")

	// canonical → Claude Code
	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	rendered := string(result.Content)
	assertContains(t, rendered, "oauth")
	assertContains(t, rendered, "my-client")
	assertContains(t, rendered, "read")
	assertContains(t, rendered, "write")
	assertContains(t, rendered, "auth.example.com")
}

func TestOAuthWarningForUnsupportedProvider(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"auth-server": {
				"command": "node",
				"args": ["server.js"],
				"oauth": {"client_id": "abc"}
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Providers that should warn about OAuth
	warnProviders := []provider.Provider{
		provider.GeminiCLI,
		provider.CopilotCLI,
		provider.Cursor,
		provider.Kiro,
		provider.RooCode,
		provider.Windsurf,
	}

	for _, prov := range warnProviders {
		t.Run(prov.Name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, prov)
			if err != nil {
				t.Fatalf("Render to %s: %v", prov.Name, err)
			}

			hasOAuthWarning := false
			for _, w := range result.Warnings {
				if strings.Contains(w, "oauth") {
					hasOAuthWarning = true
					break
				}
			}
			if !hasOAuthWarning {
				t.Errorf("expected OAuth warning for %s, got warnings: %v", prov.Name, result.Warnings)
			}
		})
	}
}

func TestOAuthNoWarningForSupportedProviders(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"auth-server": {
				"command": "node",
				"args": ["server.js"],
				"oauth": {"client_id": "abc"}
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Claude Code and OpenCode support OAuth — no warning expected
	noWarnProviders := []provider.Provider{
		provider.ClaudeCode,
		provider.OpenCode,
	}

	for _, prov := range noWarnProviders {
		t.Run(prov.Name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, prov)
			if err != nil {
				t.Fatalf("Render to %s: %v", prov.Name, err)
			}

			for _, w := range result.Warnings {
				if strings.Contains(w, "oauth") {
					t.Errorf("unexpected OAuth warning for %s: %s", prov.Name, w)
				}
			}
		})
	}
}

// --- Streamable HTTP Transport Type Mapping ---

func TestMCPCanonicalizeHttpToStreamableHTTP(t *testing.T) {
	// Claude Code / Copilot CLI use "http" for streamable HTTP transport.
	// Canonical format should normalize to "streamable-http".
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"url": "https://api.example.com/mcp",
				"type": "http"
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(canonical.Content, &cfg)

	server := cfg.MCPServers["remote"]
	assertEqual(t, "streamable-http", server.Type)
	assertEqual(t, "https://api.example.com/mcp", server.URL)
}

func TestMCPRenderClaudeStreamableHTTPToHttp(t *testing.T) {
	// Canonical "streamable-http" should render as "http" for Claude Code.
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"url": "https://api.example.com/mcp",
				"type": "streamable-http"
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["remote"]
	assertEqual(t, "http", server.Type)
}

func TestMCPRenderCopilotStreamableHTTPToHttp(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"url": "https://api.example.com/mcp",
				"type": "streamable-http"
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["remote"]
	assertEqual(t, "http", server.Type)
}

func TestMCPRenderCursorStreamableHTTPToHttp(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"url": "https://api.example.com/mcp",
				"type": "streamable-http"
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["remote"]
	assertEqual(t, "http", server.Type)
}

func TestMCPStreamableHTTPRoundTrip(t *testing.T) {
	// Claude Code config with type:"http" → canonicalize → render to Claude Code → still type:"http"
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"url": "https://api.example.com/mcp",
				"type": "http",
				"headers": {"Authorization": "Bearer tok"}
			}
		}
	}`)

	conv := &MCPConverter{}

	// Claude Code → canonical (http → streamable-http)
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var canCfg mcpConfig
	json.Unmarshal(canonical.Content, &canCfg)
	assertEqual(t, "streamable-http", canCfg.MCPServers["remote"].Type)

	// canonical → Claude Code (streamable-http → http)
	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var outCfg mcpConfig
	json.Unmarshal(result.Content, &outCfg)
	assertEqual(t, "http", outCfg.MCPServers["remote"].Type)
	assertEqual(t, "https://api.example.com/mcp", outCfg.MCPServers["remote"].URL)
	assertEqual(t, "Bearer tok", outCfg.MCPServers["remote"].Headers["Authorization"])
}

func TestWindsurfMCPCanonicalizeDisabledTools(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"disabledTools": ["create_issue", "delete_repo"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "disabledTools")
	assertContains(t, out, "create_issue")
	assertContains(t, out, "delete_repo")
}

func TestRooCodeMCPCanonicalizeDisabledTools(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"disabledTools": ["create_issue", "delete_repo"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "roo-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "disabledTools")
	assertContains(t, out, "create_issue")
	assertContains(t, out, "delete_repo")
}

func TestKiroMCPRenderDisabledTools(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"disabledTools": ["create_issue", "delete_repo"]
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "disabledTools")
	assertContains(t, out, "create_issue")
	assertContains(t, out, "delete_repo")
}

func TestWindsurfMCPRenderDisabledTools(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"disabledTools": ["create_issue"]
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "disabledTools")
	assertContains(t, out, "create_issue")
}

func TestRooCodeMCPRenderDisabledTools(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"disabledTools": ["create_issue"]
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "disabledTools")
	assertContains(t, out, "create_issue")
}

func TestClaudeMCPDropsDisabledTools(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"disabledTools": ["create_issue"]
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertNotContains(t, out, "disabledTools")
	assertNotContains(t, out, "create_issue")

	// Should warn about dropped disabledTools
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "disabledTools") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about dropped disabledTools")
	}
}

func TestDisabledToolsRoundTripKiro(t *testing.T) {
	// Windsurf → canonical → Kiro: disabledTools preserved
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"disabledTools": ["create_issue", "delete_repo"]
			}
		}
	}`)

	conv := &MCPConverter{}

	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "disabledTools")
	assertContains(t, out, "create_issue")
	assertContains(t, out, "delete_repo")
}

// --- VS Code Copilot MCP ---

func TestVSCodeCopilotMCPCanonicalize(t *testing.T) {
	input := []byte(`{
		"servers": {
			"github": {
				"type": "stdio",
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "tok123"}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "vscode-copilot")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "npx")
	assertContains(t, out, "@modelcontextprotocol/server-github")
	assertContains(t, out, "GITHUB_TOKEN")
	assertEqual(t, "mcp.json", result.Filename)
}

func TestVSCodeCopilotMCPCanonicalizeHTTPType(t *testing.T) {
	// VS Code uses "http" for streamable-http; canonical should normalize it
	input := []byte(`{
		"servers": {
			"remote": {
				"type": "http",
				"url": "https://api.example.com/mcp",
				"headers": {"Authorization": "Bearer tok"}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "vscode-copilot")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["remote"]
	assertEqual(t, "streamable-http", server.Type)
	assertEqual(t, "https://api.example.com/mcp", server.URL)
	assertEqual(t, "Bearer tok", server.Headers["Authorization"])
}

func TestVSCodeCopilotMCPCanonicalizeSSE(t *testing.T) {
	input := []byte(`{
		"servers": {
			"sse-server": {
				"type": "sse",
				"url": "https://sse.example.com/events"
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "vscode-copilot")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["sse-server"]
	assertEqual(t, "sse", server.Type)
	assertEqual(t, "https://sse.example.com/events", server.URL)
}

func TestVSCodeCopilotMCPCanonicalizeWarnings(t *testing.T) {
	// envFile, sandboxEnabled, sandbox, and inputs should produce warnings
	tr := true
	_ = tr
	input := []byte(`{
		"inputs": [
			{"type": "promptString", "id": "api_key", "description": "Enter API key", "password": true}
		],
		"servers": {
			"myserver": {
				"type": "stdio",
				"command": "node",
				"args": ["server.js"],
				"envFile": ".env",
				"sandboxEnabled": true,
				"sandbox": {"permissions": ["network"]}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "vscode-copilot")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Should have warnings for inputs, envFile, sandboxEnabled, sandbox
	if len(result.Warnings) < 3 {
		t.Fatalf("expected at least 3 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}

	allWarnings := strings.Join(result.Warnings, " | ")
	assertContains(t, allWarnings, "input variable")
	assertContains(t, allWarnings, "envFile")
	assertContains(t, allWarnings, "sandbox")
}

func TestVSCodeCopilotMCPRender(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"}
			}
		}
	}`)

	conv := &MCPConverter{}
	vscode := provider.Provider{Slug: "vscode-copilot", Name: "VS Code Copilot"}
	result, err := conv.Render(input, vscode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Should use "servers" key, not "mcpServers"
	assertContains(t, out, "servers")
	assertContains(t, out, "npx")
	assertContains(t, out, "GITHUB_TOKEN")
	// stdio should get type:"stdio"
	assertContains(t, out, `"type": "stdio"`)
	assertEqual(t, "mcp.json", result.Filename)
}

func TestVSCodeCopilotMCPRenderStreamableHTTP(t *testing.T) {
	// canonical "streamable-http" should render as "http" for VS Code
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"url": "https://api.example.com/mcp",
				"type": "streamable-http",
				"headers": {"Authorization": "Bearer tok"}
			}
		}
	}`)

	conv := &MCPConverter{}
	vscode := provider.Provider{Slug: "vscode-copilot", Name: "VS Code Copilot"}
	result, err := conv.Render(input, vscode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, `"type": "http"`)
	assertContains(t, out, "api.example.com")
}

func TestVSCodeCopilotMCPRenderDropsProviderFields(t *testing.T) {
	// Fields from other providers should produce warnings
	input := []byte(`{
		"mcpServers": {
			"test": {
				"command": "node",
				"args": ["s.js"],
				"cwd": "/app",
				"autoApprove": ["read"],
				"trust": "high",
				"includeTools": ["search"],
				"excludeTools": ["delete"],
				"disabledTools": ["exec"],
				"oauth": {"client_id": "abc"}
			}
		}
	}`)

	conv := &MCPConverter{}
	vscode := provider.Provider{Slug: "vscode-copilot", Name: "VS Code Copilot"}
	result, err := conv.Render(input, vscode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertNotContains(t, out, "cwd")
	assertNotContains(t, out, "autoApprove")
	assertNotContains(t, out, "trust")
	assertNotContains(t, out, "includeTools")
	assertNotContains(t, out, "excludeTools")
	assertNotContains(t, out, "disabledTools")

	if len(result.Warnings) < 6 {
		t.Fatalf("expected at least 6 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestVSCodeCopilotMCPRoundTrip(t *testing.T) {
	input := []byte(`{
		"servers": {
			"github": {
				"type": "stdio",
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "tok"}
			}
		}
	}`)

	conv := &MCPConverter{}
	vscode := provider.Provider{Slug: "vscode-copilot", Name: "VS Code Copilot"}

	canonical, err := conv.Canonicalize(input, "vscode-copilot")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, vscode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "servers")
	assertContains(t, out, "npx")
	assertContains(t, out, "GITHUB_TOKEN")
}

// --- Codex MCP ---

func TestCodexMCPCanonicalize(t *testing.T) {
	input := []byte(`[mcp_servers.github]
type = "stdio"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]

[mcp_servers.github.env_vars]
GITHUB_TOKEN = "tok123"
`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "npx")
	assertContains(t, out, "@modelcontextprotocol/server-github")
	assertContains(t, out, "GITHUB_TOKEN")
	// env_vars should be mapped to env
	assertContains(t, out, `"env"`)
	assertNotContains(t, out, "env_vars")
	assertEqual(t, "mcp.json", result.Filename)
}

func TestCodexMCPCanonicalizeBearerToken(t *testing.T) {
	input := []byte(`[mcp_servers.remote]
type = "http"
url = "https://api.example.com/mcp"
bearer_token_env_var = "API_TOKEN"
`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["remote"]
	assertEqual(t, "streamable-http", server.Type)
	assertEqual(t, "https://api.example.com/mcp", server.URL)
	assertEqual(t, "Bearer ${API_TOKEN}", server.Headers["Authorization"])
}

func TestCodexMCPCanonicalizeHTTPHeaders(t *testing.T) {
	input := []byte(`[mcp_servers.remote]
url = "https://api.example.com/mcp"

[mcp_servers.remote.env_http_headers]
X-Custom = "custom-value"
`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["remote"]
	assertEqual(t, "custom-value", server.Headers["X-Custom"])
}

func TestCodexMCPCanonicalizeDisabledServer(t *testing.T) {
	input := []byte(`[mcp_servers.test]
command = "node"
args = ["server.js"]
enabled = false
`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["test"]
	if !server.Disabled {
		t.Fatal("expected server to be disabled when enabled=false")
	}
}

func TestCodexMCPCanonicalizeToolFilters(t *testing.T) {
	input := []byte(`[mcp_servers.test]
command = "node"
args = ["server.js"]
enabled_tools = ["read", "search"]
disabled_tools = ["delete"]
`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["test"]
	if len(server.IncludeTools) != 2 {
		t.Fatalf("expected 2 includeTools, got %d", len(server.IncludeTools))
	}
	if len(server.ExcludeTools) != 1 {
		t.Fatalf("expected 1 excludeTools, got %d", len(server.ExcludeTools))
	}
}

func TestCodexMCPRender(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Should be TOML with mcp_servers key
	assertContains(t, out, "[mcp_servers")
	assertContains(t, out, "command = 'npx'")
	// env should be env_vars
	assertContains(t, out, "env_vars")
	assertEqual(t, "codex.toml", result.Filename)
}

func TestCodexMCPRenderStreamableHTTP(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"remote": {
				"url": "https://api.example.com/mcp",
				"type": "streamable-http",
				"headers": {"Authorization": "Bearer tok"}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// streamable-http should render as "http" for Codex
	assertContains(t, out, "type = 'http'")
	assertContains(t, out, "env_http_headers")
}

func TestCodexMCPRenderDisabled(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"test": {
				"command": "node",
				"args": ["server.js"],
				"disabled": true
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "enabled = false")
}

func TestCodexMCPRenderToolFilters(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"test": {
				"command": "node",
				"args": ["server.js"],
				"includeTools": ["read"],
				"excludeTools": ["delete"]
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "enabled_tools")
	assertContains(t, out, "disabled_tools")
}

func TestCodexMCPRenderWarnings(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"test": {
				"command": "node",
				"args": ["server.js"],
				"cwd": "/app",
				"autoApprove": ["read"],
				"trust": "high",
				"oauth": {"client_id": "abc"}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) < 3 {
		t.Fatalf("expected at least 3 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}

	allWarnings := strings.Join(result.Warnings, " | ")
	assertContains(t, allWarnings, "cwd")
	assertContains(t, allWarnings, "autoApprove")
	assertContains(t, allWarnings, "trust")
}

// --- Amp MCP ---

func TestAmpMCPCanonicalize(t *testing.T) {
	input := []byte(`{
		"amp.mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "tok"}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "amp")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "npx")
	assertContains(t, out, "GITHUB_TOKEN")
	assertEqual(t, "mcp.json", result.Filename)
}

func TestAmpMCPCanonicalizeURLServer(t *testing.T) {
	input := []byte(`{
		"amp.mcpServers": {
			"remote": {
				"url": "https://api.example.com/mcp",
				"headers": {"Authorization": "Bearer tok"},
				"includeTools": ["search"]
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "amp")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["remote"]
	assertEqual(t, "streamable-http", server.Type)
	assertEqual(t, "https://api.example.com/mcp", server.URL)
	if len(server.IncludeTools) != 1 || server.IncludeTools[0] != "search" {
		t.Fatalf("expected includeTools=[search], got %v", server.IncludeTools)
	}
}

func TestAmpMCPRender(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "token"},
				"includeTools": ["search"]
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.Amp)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "amp.mcpServers")
	assertContains(t, out, "npx")
	assertContains(t, out, "GITHUB_TOKEN")
	assertContains(t, out, "includeTools")
	assertEqual(t, "settings.json", result.Filename)
}

func TestAmpMCPRenderWarnings(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"test": {
				"command": "node",
				"args": ["server.js"],
				"cwd": "/app",
				"autoApprove": ["read"],
				"trust": "high",
				"excludeTools": ["delete"],
				"disabledTools": ["exec"],
				"oauth": {"client_id": "abc"},
				"disabled": true
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Render(input, provider.Amp)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) < 6 {
		t.Fatalf("expected at least 6 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}

	allWarnings := strings.Join(result.Warnings, " | ")
	assertContains(t, allWarnings, "cwd")
	assertContains(t, allWarnings, "autoApprove")
	assertContains(t, allWarnings, "trust")
	assertContains(t, allWarnings, "excludeTools")
	assertContains(t, allWarnings, "disabledTools")
	assertContains(t, allWarnings, "disabled")
}

func TestOpenCodeToClaudeOAuthPreserved(t *testing.T) {
	input := []byte(`{
		"mcp": {
			"auth-server": {
				"type": "remote",
				"url": "https://api.example.com/mcp",
				"oauth": {
					"client_id": "oc-client",
					"scopes": ["api"],
					"token_url": "https://auth.example.com/token"
				}
			}
		}
	}`)

	conv := &MCPConverter{}

	// OpenCode → canonical
	canonical, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "oauth")
	assertContains(t, out, "oc-client")

	// canonical → Claude Code
	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	rendered := string(result.Content)
	assertContains(t, rendered, "oauth")
	assertContains(t, rendered, "oc-client")
	assertContains(t, rendered, "api")
	assertContains(t, rendered, "auth.example.com")

	// No warnings — both providers support OAuth
	for _, w := range result.Warnings {
		if strings.Contains(w, "oauth") {
			t.Errorf("unexpected OAuth warning: %s", w)
		}
	}
}
