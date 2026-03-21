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

func TestZedMCPHTTPServerDropped(t *testing.T) {
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

	result, err := conv.Render(canonical.Content, provider.Zed)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about HTTP transport not supported")
	}
	assertContains(t, result.Warnings[0], "remoteapi")

	out := string(result.Content)
	assertNotContains(t, out, "remoteapi")
	assertNotContains(t, out, "api.example.com")
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

func TestClineMCPHTTPServerDropped(t *testing.T) {
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

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about HTTP transport not supported")
	}
	assertContains(t, result.Warnings[0], "remoteapi")

	out := string(result.Content)
	assertNotContains(t, out, "remoteapi")
	assertNotContains(t, out, "api.example.com")
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

func TestRooCodeMCPDropsCwd(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"local": {
				"command": "node",
				"args": ["server.js"],
				"cwd": "/app"
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
	assertNotContains(t, out, "cwd")
	assertNotContains(t, out, "/app")
	assertContains(t, out, "node")

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about dropped cwd")
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
	assertContains(t, out, "autoApprove")
	assertContains(t, out, "search_repositories")
	assertEqual(t, "mcp.json", result.Filename)
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
	assertContains(t, out, "autoApprove")
	assertContains(t, out, "search_repositories")
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
