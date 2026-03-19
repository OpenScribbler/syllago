package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
)

func TestInstallMCP_WhitelistsFields(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a mock MCP item with extra unknown fields
	itemDir := filepath.Join(tmpDir, "test-mcp")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	configData := map[string]interface{}{
		"type":    "stdio",
		"command": "node",
		"args":    []string{"server.js"},
		"env": map[string]string{
			"API_KEY": "placeholder",
		},
		"malicious_field":  "evil data",
		"unexpected_key":   "should be dropped",
		"_internal_config": "not for user config",
	}

	configJSON, err := json.Marshal(configData)
	if err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(itemDir, "config.json")
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatal(err)
	}

	// Create mock item
	item := catalog.ContentItem{
		Name: "test-server",
		Type: catalog.MCP,
		Path: itemDir,
	}

	// Create mock provider config file
	configFile := filepath.Join(tmpDir, ".claude.json")
	if err := os.WriteFile(configFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	prov := provider.Provider{
		Slug: "test-provider",
	}

	// Override mcpConfigPath for test
	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	// Install
	if _, err := installMCP(item, prov, tmpDir); err != nil {
		t.Fatalf("installMCP failed: %v", err)
	}

	// Read back config
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}

	// Check that only whitelisted fields exist
	serverConfig := gjson.GetBytes(data, "mcpServers.test-server")
	if !serverConfig.Exists() {
		t.Fatal("server config not found")
	}

	// Should have whitelisted fields
	if serverConfig.Get("type").String() != "stdio" {
		t.Error("type field missing or wrong")
	}
	if serverConfig.Get("command").String() != "node" {
		t.Error("command field missing or wrong")
	}

	// Should NOT have unknown fields
	if serverConfig.Get("malicious_field").Exists() {
		t.Error("malicious_field should have been dropped")
	}
	if serverConfig.Get("unexpected_key").Exists() {
		t.Error("unexpected_key should have been dropped")
	}
	if serverConfig.Get("_internal_config").Exists() {
		t.Error("_internal_config should have been dropped")
	}

	// Should NOT have _syllago marker (removed — tracking is via installed.json)
	if serverConfig.Get("_syllago").Exists() {
		t.Error("_syllago marker should not be present")
	}

	// Verify installed.json was written
	inst, err := LoadInstalled(tmpDir)
	if err != nil {
		t.Fatalf("loading installed.json: %v", err)
	}
	if inst.FindMCP("test-server") < 0 {
		t.Error("test-server not found in installed.json")
	}
}

func TestMCPConfigPath_ProjectScoped(t *testing.T) {
	repoRoot := "/tmp/test-project"

	tests := []struct {
		slug     string
		wantPath string
	}{
		{"copilot-cli", filepath.Join(repoRoot, ".copilot", "mcp.json")},
		{"kiro", filepath.Join(repoRoot, ".kiro", "settings", "mcp.json")},
		{"opencode", filepath.Join(repoRoot, "opencode.json")},
		{"cline", filepath.Join(repoRoot, ".vscode", "mcp.json")},
		{"roo-code", filepath.Join(repoRoot, ".roo", "mcp.json")},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			prov := provider.Provider{Slug: tt.slug}
			got, err := mcpConfigPathImpl(prov, repoRoot)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantPath {
				t.Errorf("want %s, got %s", tt.wantPath, got)
			}
		})
	}
}

func TestMCPConfigKey_Zed(t *testing.T) {
	zed := provider.Provider{Slug: "zed"}
	claude := provider.Provider{Slug: "claude-code"}

	if got := MCPConfigKey(zed); got != "context_servers" {
		t.Errorf("zed key: want context_servers, got %s", got)
	}
	if got := MCPConfigKey(claude); got != "mcpServers" {
		t.Errorf("claude key: want mcpServers, got %s", got)
	}
}

func TestInstallMCP_ZedUsesContextServers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create MCP item
	itemDir := filepath.Join(tmpDir, "test-mcp")
	os.MkdirAll(itemDir, 0755)
	configJSON, _ := json.Marshal(map[string]interface{}{
		"type":    "stdio",
		"command": "node",
		"args":    []string{"server.js"},
	})
	os.WriteFile(filepath.Join(itemDir, "config.json"), configJSON, 0644)

	item := catalog.ContentItem{Name: "test-server", Type: catalog.MCP, Path: itemDir}

	// Create mock Zed config file
	configFile := filepath.Join(tmpDir, "settings.json")
	os.WriteFile(configFile, []byte("{}"), 0644)

	prov := provider.Provider{Slug: "zed"}

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	_, err := installMCP(item, prov, tmpDir)
	if err != nil {
		t.Fatalf("installMCP for zed: %v", err)
	}

	data, _ := os.ReadFile(configFile)
	// Zed should use context_servers, not mcpServers
	serverConfig := gjson.GetBytes(data, "context_servers.test-server")
	if !serverConfig.Exists() {
		t.Fatal("expected context_servers.test-server in Zed config")
	}
	if gjson.GetBytes(data, "mcpServers.test-server").Exists() {
		t.Fatal("Zed should NOT use mcpServers key")
	}
}

func TestInstallMCP_OpenCodeStripsJSONCComments(t *testing.T) {
	tmpDir := t.TempDir()

	// Create MCP item
	itemDir := filepath.Join(tmpDir, "test-mcp")
	os.MkdirAll(itemDir, 0755)
	configJSON, _ := json.Marshal(map[string]interface{}{
		"type":    "stdio",
		"command": "python",
		"args":    []string{"-m", "server"},
	})
	os.WriteFile(filepath.Join(itemDir, "config.json"), configJSON, 0644)

	item := catalog.ContentItem{Name: "my-server", Type: catalog.MCP, Path: itemDir}

	// Create OpenCode config with JSONC comments
	configFile := filepath.Join(tmpDir, "opencode.json")
	jsoncContent := `{
  // This is a JSONC comment
  "mcpServers": {
    /* block comment */
    "existing": {"command": "existing-server"}
  }
}`
	os.WriteFile(configFile, []byte(jsoncContent), 0644)

	prov := provider.Provider{Slug: "opencode"}

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	_, err := installMCP(item, prov, tmpDir)
	if err != nil {
		t.Fatalf("installMCP for opencode: %v", err)
	}

	data, _ := os.ReadFile(configFile)
	// New server should be merged
	newServer := gjson.GetBytes(data, "mcpServers.my-server")
	if !newServer.Exists() {
		t.Fatal("expected mcpServers.my-server after merge")
	}
	// Existing server should be preserved
	existing := gjson.GetBytes(data, "mcpServers.existing")
	if !existing.Exists() {
		t.Fatal("existing server should be preserved after merge")
	}
}

func TestInstallMCP_NestedFormat(t *testing.T) {
	// Not parallel — mutates package-level mcpConfigPath
	tmpDir := t.TempDir()

	// Create MCP item with nested format (mcpServers wrapper)
	itemDir := filepath.Join(tmpDir, "kitchen-sink-mcp")
	os.MkdirAll(itemDir, 0755)
	nestedConfig := `{
		"mcpServers": {
			"stdio-server": {
				"type": "stdio",
				"command": "npx",
				"args": ["-y", "@example/server"],
				"env": {"API_KEY": "test-key"}
			},
			"http-server": {
				"url": "https://mcp.example.com",
				"type": "streamable-http"
			}
		}
	}`
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(nestedConfig), 0644)

	item := catalog.ContentItem{Name: "kitchen-sink-mcp", Type: catalog.MCP, Path: itemDir}

	// Create target config file with existing server
	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte(`{"mcpServers":{"existing":{"command":"keep-me"}}}`), 0644)

	prov := provider.Provider{Slug: "claude-code"}

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	_, err := installMCP(item, prov, tmpDir)
	if err != nil {
		t.Fatalf("installMCP failed: %v", err)
	}

	data, _ := os.ReadFile(configFile)

	// Both servers from the nested config should be present
	stdio := gjson.GetBytes(data, "mcpServers.stdio-server")
	if !stdio.Exists() {
		t.Fatal("expected mcpServers.stdio-server")
	}
	if stdio.Get("command").String() != "npx" {
		t.Error("stdio-server command should be npx")
	}
	if stdio.Get("env.API_KEY").String() != "test-key" {
		t.Error("stdio-server env.API_KEY should be test-key")
	}

	http := gjson.GetBytes(data, `mcpServers.http-server`)
	if !http.Exists() {
		t.Fatal("expected mcpServers.http-server")
	}
	if http.Get("url").String() != "https://mcp.example.com" {
		t.Error("http-server url should be https://mcp.example.com")
	}

	// Existing server should survive
	if !gjson.GetBytes(data, "mcpServers.existing").Exists() {
		t.Fatal("existing server should survive merge")
	}

	// Item name should NOT appear as a server key
	if gjson.GetBytes(data, "mcpServers.kitchen-sink-mcp").Exists() {
		t.Error("item name should not be used as server key for nested format")
	}

	// installed.json should track the actual server names
	inst, _ := LoadInstalled(tmpDir)
	idx := inst.FindMCP("kitchen-sink-mcp")
	if idx < 0 {
		t.Fatal("kitchen-sink-mcp not found in installed.json")
	}
	names := inst.MCP[idx].ServerNames
	if len(names) != 2 {
		t.Fatalf("expected 2 server names, got %d", len(names))
	}
}

func TestUninstallMCP_NestedFormat(t *testing.T) {
	// Not parallel — mutates package-level mcpConfigPath
	tmpDir := t.TempDir()

	// Create MCP item with nested format
	itemDir := filepath.Join(tmpDir, "multi-mcp")
	os.MkdirAll(itemDir, 0755)
	nestedConfig := `{
		"mcpServers": {
			"server-a": {"command": "a"},
			"server-b": {"command": "b"}
		}
	}`
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(nestedConfig), 0644)

	item := catalog.ContentItem{Name: "multi-mcp", Type: catalog.MCP, Path: itemDir}

	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte(`{"mcpServers":{"keep-me":{"command":"stay"}}}`), 0644)

	prov := provider.Provider{Slug: "claude-code"}

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	// Install
	_, err := installMCP(item, prov, tmpDir)
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	// Verify both servers exist
	data, _ := os.ReadFile(configFile)
	if !gjson.GetBytes(data, "mcpServers.server-a").Exists() {
		t.Fatal("server-a should exist after install")
	}

	// Uninstall
	_, err = uninstallMCP(item, prov, tmpDir)
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	data, _ = os.ReadFile(configFile)

	// Both servers should be gone
	if gjson.GetBytes(data, "mcpServers.server-a").Exists() {
		t.Error("server-a should be removed")
	}
	if gjson.GetBytes(data, "mcpServers.server-b").Exists() {
		t.Error("server-b should be removed")
	}

	// Existing server should survive
	if !gjson.GetBytes(data, "mcpServers.keep-me").Exists() {
		t.Fatal("keep-me should survive uninstall")
	}

	// installed.json should be clean
	inst, _ := LoadInstalled(tmpDir)
	if inst.FindMCP("multi-mcp") >= 0 {
		t.Error("multi-mcp should be removed from installed.json")
	}
}
