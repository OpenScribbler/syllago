package converter

import (
	"encoding/json"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/provider"
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
