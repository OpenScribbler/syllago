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
	// Not parallel — mutates package-level mcpConfigPath
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
		{"cline", provider.ClineMCPSettingsPath()},
		{"roo-code", filepath.Join(repoRoot, ".roo", "mcp.json")},
		// Cursor is documented at both the global and project path; we install
		// to the project path for parity with other per-repo providers. If this
		// case disappears, production install for slug=cursor regresses to the
		// "MCP config path not defined" error path (see bead syllago-4w5xy).
		{"cursor", filepath.Join(repoRoot, ".cursor", "mcp.json")},
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

// TestMCPConfigPath_HomeScoped covers Windsurf, whose MCP config lives at
// ~/.codeium/windsurf/mcp_config.json with no project-local alternative.
// Unlike the project-scoped cases this one depends on os.UserHomeDir(), so
// we assert via suffix rather than hard-coding a home path.
func TestMCPConfigPath_HomeScoped(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir: %v", err)
	}

	tests := []struct {
		slug     string
		wantPath string
	}{
		{"windsurf", filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			prov := provider.Provider{Slug: tt.slug}
			got, err := mcpConfigPathImpl(prov, "/tmp/irrelevant-repo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantPath {
				t.Errorf("want %s, got %s", tt.wantPath, got)
			}
		})
	}
}

// TestInstallMCP_Cursor_ProductionPath exercises the real mcpConfigPath
// function (no test-seam override) so the switch case for slug=cursor is
// covered end-to-end. Without this, the unit tests above assert the switch
// maps correctly, but installMCP itself could still regress if a future
// refactor rerouted its path lookup through a different function.
func TestInstallMCP_Cursor_ProductionPath(t *testing.T) {
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "cursor-mcp")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	configJSON, _ := json.Marshal(map[string]interface{}{
		"command": "npx",
		"args":    []string{"-y", "@modelcontextprotocol/server-github"},
	})
	if err := os.WriteFile(filepath.Join(itemDir, "config.json"), configJSON, 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-create the project-local MCP config file at the canonical path.
	cfgDir := filepath.Join(tmpDir, ".cursor")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "mcp.json")
	if err := os.WriteFile(cfgPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{Name: "github", Type: catalog.MCP, Path: itemDir}
	prov := provider.Provider{Slug: "cursor", Name: "Cursor"}

	if _, err := installMCP(item, prov, tmpDir); err != nil {
		t.Fatalf("installMCP via real path: %v — regression; slug=cursor no longer maps through mcpConfigPathImpl", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("cursor MCP config was not written at the canonical path %s: %v", cfgPath, err)
	}
	if got := gjson.GetBytes(data, "mcpServers.github.command").String(); got != "npx" {
		t.Errorf("mcpServers.github.command = %q, want npx (merge via real path broke)", got)
	}
}

// TestInstallMCP_Windsurf_ProductionPath exercises slug=windsurf without
// the test seam. Windsurf's MCP config lives at the user's home dir, so the
// test has to HOME-swap to avoid polluting the real developer machine.
func TestInstallMCP_Windsurf_ProductionPath(t *testing.T) {
	tmpDir := t.TempDir()

	fakeHome := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(fakeHome, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fakeHome)

	itemDir := filepath.Join(tmpDir, "windsurf-mcp")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	configJSON, _ := json.Marshal(map[string]interface{}{
		"command": "python",
		"args":    []string{"-m", "mcp_local"},
	})
	if err := os.WriteFile(filepath.Join(itemDir, "config.json"), configJSON, 0644); err != nil {
		t.Fatal(err)
	}

	cfgDir := filepath.Join(fakeHome, ".codeium", "windsurf")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "mcp_config.json")
	if err := os.WriteFile(cfgPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{Name: "local-py", Type: catalog.MCP, Path: itemDir}
	prov := provider.Provider{Slug: "windsurf", Name: "Windsurf"}

	if _, err := installMCP(item, prov, tmpDir); err != nil {
		t.Fatalf("installMCP via real path: %v — regression; slug=windsurf no longer maps through mcpConfigPathImpl", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("windsurf MCP config was not written at the canonical path %s: %v", cfgPath, err)
	}
	if got := gjson.GetBytes(data, "mcpServers.local-py.command").String(); got != "python" {
		t.Errorf("mcpServers.local-py.command = %q, want python (merge via real path broke)", got)
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

func TestInstallMCP_RejectsInvalidServerNames(t *testing.T) {
	// Not parallel — mutates package-level mcpConfigPath
	tests := []struct {
		name       string
		serverName string
	}{
		{"dot traversal", "evil..path"},
		{"dots in name", "server.name"},
		{"path separator", "server/name"},
		{"backslash", "server\\name"},
		{"space", "server name"},
		{"leading dash", "-server"},
		{"special chars", "server@name"},
		{"empty via nested", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create MCP item with a malicious nested server name
			itemDir := filepath.Join(tmpDir, "bad-mcp")
			os.MkdirAll(itemDir, 0755)

			// Build nested config with the malicious server name as a JSON key
			nestedConfig, _ := json.Marshal(map[string]interface{}{
				"mcpServers": map[string]interface{}{
					tt.serverName: map[string]interface{}{
						"command": "evil",
					},
				},
			})
			os.WriteFile(filepath.Join(itemDir, "config.json"), nestedConfig, 0644)

			item := catalog.ContentItem{Name: "bad-mcp", Type: catalog.MCP, Path: itemDir}

			configFile := filepath.Join(tmpDir, ".claude.json")
			os.WriteFile(configFile, []byte("{}"), 0644)

			prov := provider.Provider{Slug: "claude-code"}

			originalFunc := mcpConfigPath
			mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
				return configFile, nil
			}
			defer func() { mcpConfigPath = originalFunc }()

			_, err := installMCP(item, prov, tmpDir)
			if err == nil {
				t.Fatalf("expected error for server name %q, got nil", tt.serverName)
			}
		})
	}
}

func TestInstallMCP_RejectsInvalidFlatItemName(t *testing.T) {
	// Flat format: item.Name is used as the server key when config.json
	// doesn't have a nested wrapper. Validate it too.
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "bad-mcp")
	os.MkdirAll(itemDir, 0755)

	// Flat config (no mcpServers wrapper)
	flatConfig, _ := json.Marshal(map[string]interface{}{
		"command": "node",
		"args":    []string{"server.js"},
	})
	os.WriteFile(filepath.Join(itemDir, "config.json"), flatConfig, 0644)

	// Use a name with dots — would be an sjson path injection
	item := catalog.ContentItem{Name: "evil..path", Type: catalog.MCP, Path: itemDir}

	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte("{}"), 0644)

	prov := provider.Provider{Slug: "claude-code"}

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	_, err := installMCP(item, prov, tmpDir)
	if err == nil {
		t.Fatal("expected error for item name with dots, got nil")
	}
}

func TestParseMCPConfig_Valid(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "test-mcp")
	os.MkdirAll(itemDir, 0755)
	configData := `{"type":"stdio","command":"node","args":["server.js"],"env":{"API_KEY":"test"}}`
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(configData), 0644)

	cfg, err := ParseMCPConfig(itemDir)
	if err != nil {
		t.Fatalf("ParseMCPConfig: %v", err)
	}
	if cfg.Type != "stdio" {
		t.Errorf("Type = %q, want stdio", cfg.Type)
	}
	if cfg.Command != "node" {
		t.Errorf("Command = %q, want node", cfg.Command)
	}
	if len(cfg.Args) != 1 || cfg.Args[0] != "server.js" {
		t.Errorf("Args = %v, want [server.js]", cfg.Args)
	}
	if cfg.Env["API_KEY"] != "test" {
		t.Errorf("Env[API_KEY] = %q, want test", cfg.Env["API_KEY"])
	}
}

func TestParseMCPConfig_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := ParseMCPConfig(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing config.json")
	}
}

func TestParseMCPConfig_InvalidJSON(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte("{invalid}"), 0644)
	_, err := ParseMCPConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseMCPConfig_URLType(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configData := `{"url":"https://mcp.example.com","type":"streamable-http"}`
	os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(configData), 0644)

	cfg, err := ParseMCPConfig(tmpDir)
	if err != nil {
		t.Fatalf("ParseMCPConfig: %v", err)
	}
	if cfg.URL != "https://mcp.example.com" {
		t.Errorf("URL = %q, want https://mcp.example.com", cfg.URL)
	}
}

func TestCheckEnvVars(t *testing.T) {
	// Not parallel — uses os.Setenv
	t.Setenv("SYLLAGO_TEST_SET_VAR", "value")

	cfg := &MCPConfig{
		Env: map[string]string{
			"SYLLAGO_TEST_SET_VAR":   "placeholder",
			"SYLLAGO_TEST_UNSET_VAR": "placeholder",
		},
	}

	result := CheckEnvVars(cfg)
	if !result["SYLLAGO_TEST_SET_VAR"] {
		t.Error("SYLLAGO_TEST_SET_VAR should be set")
	}
	if result["SYLLAGO_TEST_UNSET_VAR"] {
		t.Error("SYLLAGO_TEST_UNSET_VAR should not be set")
	}
}

func TestCheckEnvVars_NoEnv(t *testing.T) {
	t.Parallel()
	cfg := &MCPConfig{}
	result := CheckEnvVars(cfg)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestMCPConfigPathFor(t *testing.T) {
	// Not parallel — depends on os.UserHomeDir
	prov := provider.Provider{Slug: "claude-code"}
	path, err := MCPConfigPathFor(prov, "/repo")
	if err != nil {
		t.Fatalf("MCPConfigPathFor: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
}

func TestMCPConfigPathFor_UnknownProvider(t *testing.T) {
	t.Parallel()
	prov := provider.Provider{Slug: "unknown-provider"}
	_, err := mcpConfigPathImpl(prov, "/repo")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestCheckMCPStatus_InstalledViaInstalledJSON(t *testing.T) {
	// Not parallel — mutates package-level mcpConfigPath
	tmpDir := t.TempDir()

	// Create provider config file with the server entry
	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte(`{"mcpServers":{"my-server":{"command":"node"}}}`), 0644)

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	prov := provider.Provider{Slug: "claude-code"}

	// Record in installed.json
	inst := &Installed{
		MCP: []InstalledMCP{
			{Name: "my-server", ServerNames: []string{"my-server"}, Source: "export"},
		},
	}
	os.MkdirAll(filepath.Join(tmpDir, ".syllago"), 0755)
	SaveInstalled(tmpDir, inst)

	item := catalog.ContentItem{Name: "my-server", Type: catalog.MCP}
	status := checkMCPStatus(item, prov, tmpDir)
	if status != StatusInstalled {
		t.Errorf("expected StatusInstalled, got %v", status)
	}
}

func TestCheckMCPStatus_NotInstalled(t *testing.T) {
	// Not parallel — mutates package-level mcpConfigPath
	tmpDir := t.TempDir()

	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte(`{"mcpServers":{}}`), 0644)

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	prov := provider.Provider{Slug: "claude-code"}

	item := catalog.ContentItem{Name: "absent-server", Type: catalog.MCP}
	status := checkMCPStatus(item, prov, tmpDir)
	if status != StatusNotInstalled {
		t.Errorf("expected StatusNotInstalled, got %v", status)
	}
}

func TestCheckMCPStatus_FallbackByItemName(t *testing.T) {
	// Not parallel — mutates package-level mcpConfigPath
	tmpDir := t.TempDir()

	// Server exists in config but NOT in installed.json
	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte(`{"mcpServers":{"legacy-server":{"command":"node"}}}`), 0644)

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	prov := provider.Provider{Slug: "claude-code"}

	item := catalog.ContentItem{Name: "legacy-server", Type: catalog.MCP}
	status := checkMCPStatus(item, prov, tmpDir)
	if status != StatusInstalled {
		t.Errorf("expected StatusInstalled via fallback, got %v", status)
	}
}

func TestCheckMCPStatus_MissingConfigFile(t *testing.T) {
	// Not parallel — mutates package-level mcpConfigPath
	tmpDir := t.TempDir()

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return filepath.Join(tmpDir, "nonexistent.json"), nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	prov := provider.Provider{Slug: "claude-code"}

	// readMCPConfig returns {} for missing files, so status should be NotInstalled
	item := catalog.ContentItem{Name: "test", Type: catalog.MCP}
	status := checkMCPStatus(item, prov, tmpDir)
	if status != StatusNotInstalled {
		t.Errorf("expected StatusNotInstalled for missing config, got %v", status)
	}
}

func TestUninstallMCP_NotInstalledBySyllago(t *testing.T) {
	// Not parallel — mutates package-level mcpConfigPath
	tmpDir := t.TempDir()

	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte(`{"mcpServers":{"foreign":{"command":"node"}}}`), 0644)

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	prov := provider.Provider{Slug: "claude-code"}
	item := catalog.ContentItem{Name: "foreign", Type: catalog.MCP, Path: tmpDir}

	// Not in installed.json — should fail
	_, err := uninstallMCP(item, prov, tmpDir)
	if err == nil {
		t.Fatal("expected error for item not installed by syllago")
	}
}

func TestCheckMCPStatus_InstalledWithLegacyNoServerNames(t *testing.T) {
	// Not parallel — mutates package-level mcpConfigPath
	tmpDir := t.TempDir()

	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte(`{"mcpServers":{"legacy":{"command":"node"}}}`), 0644)

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	prov := provider.Provider{Slug: "claude-code"}

	// Record in installed.json WITHOUT ServerNames (legacy format)
	inst := &Installed{
		MCP: []InstalledMCP{
			{Name: "legacy", Source: "export"}, // no ServerNames
		},
	}
	os.MkdirAll(filepath.Join(tmpDir, ".syllago"), 0755)
	SaveInstalled(tmpDir, inst)

	item := catalog.ContentItem{Name: "legacy", Type: catalog.MCP}
	status := checkMCPStatus(item, prov, tmpDir)
	if status != StatusInstalled {
		t.Errorf("expected StatusInstalled for legacy (no ServerNames), got %v", status)
	}
}

func TestExtractServerEntries_FlatFormat(t *testing.T) {
	t.Parallel()
	rawData := []byte(`{"command":"node","args":["server.js"],"env":{"KEY":"val"}}`)
	entries, err := ExtractServerEntries(rawData, "my-server", "mcpServers")
	if err != nil {
		t.Fatalf("ExtractServerEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if _, ok := entries["my-server"]; !ok {
		t.Error("expected entry keyed by item name")
	}
}

func TestExtractServerEntries_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := ExtractServerEntries([]byte("{invalid}"), "test", "mcpServers")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
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

func TestFindMCPLocations_SettingsJSON(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Create a project with .claude/settings.json containing mcpServers.
	claudeDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{
		"mcpServers": {"my-server": {"command": "node"}}
	}`), 0644)

	prov := provider.Provider{
		Name:      "Claude Code",
		Slug:      "claude-code",
		ConfigDir: ".claude",
	}

	locs := FindMCPLocations(prov, tmp, "")
	found := false
	for _, loc := range locs {
		if loc.Scope == ScopeProject && loc.JSONKey == "mcpServers" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected project-scoped MCP location from settings.json, got %d locations", len(locs))
	}
}

func TestFindMCPLocations_NoMCPKey(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Create settings.json without mcpServers.
	claudeDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{
		"hooks": {"PreToolUse": []}
	}`), 0644)

	prov := provider.Provider{
		Name:      "Claude Code",
		Slug:      "claude-code",
		ConfigDir: ".claude",
	}

	locs := FindMCPLocations(prov, tmp, "")
	// Should not include the settings.json since it has no mcpServers key.
	for _, loc := range locs {
		if loc.Path == filepath.Join(claudeDir, "settings.json") {
			t.Error("settings.json without mcpServers should not be included")
		}
	}
}

func TestFindMCPLocations_DotMcpJSON(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Create .mcp.json in project root (Claude Code project-level MCP).
	os.WriteFile(filepath.Join(tmp, ".mcp.json"), []byte(`{
		"mcpServers": {"test-server": {"command": "npx", "args": ["test"]}}
	}`), 0644)

	prov := provider.Provider{
		Name:      "Claude Code",
		Slug:      "claude-code",
		ConfigDir: ".claude",
	}

	locs := FindMCPLocations(prov, tmp, "")
	found := false
	for _, loc := range locs {
		if loc.Path == filepath.Join(tmp, ".mcp.json") && loc.Scope == ScopeProject {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected .mcp.json to be found as project-scoped MCP location")
	}
}

func TestFindMCPLocations_NoDuplicates(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Don't create any MCP files.
	prov := provider.Provider{
		Name:      "Claude Code",
		Slug:      "claude-code",
		ConfigDir: ".claude",
	}

	locs := FindMCPLocations(prov, tmp, "")
	// With no files on disk, should return empty.
	seen := make(map[string]bool)
	for _, loc := range locs {
		if seen[loc.Path] {
			t.Errorf("duplicate path in MCP locations: %s", loc.Path)
		}
		seen[loc.Path] = true
	}
}

func TestInstallMCP_PerServer(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a nested config with two servers.
	itemDir := filepath.Join(tmpDir, "multi-mcp")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{
		"mcpServers": {
			"server-a": {"command": "node", "args": ["a.js"]},
			"server-b": {"url": "https://b.example.com"}
		}
	}`), 0644)

	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte("{}"), 0644)

	prov := provider.Provider{Slug: "test-provider"}
	origPath := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origPath }()

	// Install only server-a (per-server).
	itemA := catalog.ContentItem{
		Name:      "server-a",
		Type:      catalog.MCP,
		Path:      itemDir,
		ServerKey: "server-a",
	}
	if _, err := installMCP(itemA, prov, tmpDir); err != nil {
		t.Fatalf("installMCP server-a: %v", err)
	}

	// Verify only server-a is in provider config.
	data, _ := os.ReadFile(configFile)
	if !gjson.GetBytes(data, "mcpServers.server-a").Exists() {
		t.Error("server-a should be in provider config")
	}
	if gjson.GetBytes(data, "mcpServers.server-b").Exists() {
		t.Error("server-b should NOT be in provider config yet")
	}

	// Verify installed.json has per-server entry.
	inst, _ := LoadInstalled(tmpDir)
	idx := inst.FindMCPByServerKey("server-a", "server-a")
	if idx < 0 {
		t.Fatal("server-a not found in installed.json by server key")
	}
	if inst.MCP[idx].ServerKey != "server-a" {
		t.Errorf("ServerKey = %q, want %q", inst.MCP[idx].ServerKey, "server-a")
	}

	// Now install server-b independently.
	itemB := catalog.ContentItem{
		Name:      "server-b",
		Type:      catalog.MCP,
		Path:      itemDir,
		ServerKey: "server-b",
	}
	if _, err := installMCP(itemB, prov, tmpDir); err != nil {
		t.Fatalf("installMCP server-b: %v", err)
	}

	data, _ = os.ReadFile(configFile)
	if !gjson.GetBytes(data, "mcpServers.server-a").Exists() {
		t.Error("server-a should still be in provider config")
	}
	if !gjson.GetBytes(data, "mcpServers.server-b").Exists() {
		t.Error("server-b should now be in provider config")
	}

	// Verify both are tracked separately.
	inst, _ = LoadInstalled(tmpDir)
	if inst.FindMCPByServerKey("server-a", "server-a") < 0 {
		t.Error("server-a should still be tracked")
	}
	if inst.FindMCPByServerKey("server-b", "server-b") < 0 {
		t.Error("server-b should be tracked")
	}
}

func TestUninstallMCP_PerServer(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up config with both servers installed.
	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte(`{
		"mcpServers": {
			"server-a": {"command": "node", "args": ["a.js"]},
			"server-b": {"url": "https://b.example.com"}
		}
	}`), 0644)

	// Set up installed.json with per-server entries.
	inst := &Installed{
		MCP: []InstalledMCP{
			{Name: "server-a", ServerKey: "server-a", Source: "export"},
			{Name: "server-b", ServerKey: "server-b", Source: "export"},
		},
	}
	SaveInstalled(tmpDir, inst)

	prov := provider.Provider{Slug: "test-provider"}
	origPath := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origPath }()

	// Uninstall server-a only.
	itemA := catalog.ContentItem{
		Name:      "server-a",
		Type:      catalog.MCP,
		Path:      tmpDir,
		ServerKey: "server-a",
	}
	if _, err := uninstallMCP(itemA, prov, tmpDir); err != nil {
		t.Fatalf("uninstallMCP server-a: %v", err)
	}

	// server-a gone, server-b remains.
	data, _ := os.ReadFile(configFile)
	if gjson.GetBytes(data, "mcpServers.server-a").Exists() {
		t.Error("server-a should be removed from provider config")
	}
	if !gjson.GetBytes(data, "mcpServers.server-b").Exists() {
		t.Error("server-b should still be in provider config")
	}

	inst, _ = LoadInstalled(tmpDir)
	if inst.FindMCPByServerKey("server-a", "server-a") >= 0 {
		t.Error("server-a should be removed from installed.json")
	}
	if inst.FindMCPByServerKey("server-b", "server-b") < 0 {
		t.Error("server-b should still be in installed.json")
	}
}

func TestCheckMCPStatus_PerServer(t *testing.T) {
	tmpDir := t.TempDir()

	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte(`{
		"mcpServers": {
			"server-a": {"command": "node"}
		}
	}`), 0644)

	prov := provider.Provider{Slug: "test-provider"}
	origPath := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origPath }()

	itemA := catalog.ContentItem{Name: "server-a", Type: catalog.MCP, ServerKey: "server-a"}
	if got := checkMCPStatus(itemA, prov, tmpDir); got != StatusInstalled {
		t.Errorf("server-a status = %v, want StatusInstalled", got)
	}

	itemB := catalog.ContentItem{Name: "server-b", Type: catalog.MCP, ServerKey: "server-b"}
	if got := checkMCPStatus(itemB, prov, tmpDir); got != StatusNotInstalled {
		t.Errorf("server-b status = %v, want StatusNotInstalled", got)
	}
}

func TestParseMCPServerConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(`{
		"mcpServers": {
			"server-a": {"command": "node", "args": ["a.js"], "env": {"KEY": "val"}},
			"server-b": {"url": "https://b.example.com", "type": "streamable-http"}
		}
	}`), 0644)

	t.Run("extracts specific server", func(t *testing.T) {
		cfg, err := ParseMCPServerConfig(tmpDir, "server-a")
		if err != nil {
			t.Fatalf("ParseMCPServerConfig: %v", err)
		}
		if cfg.Command != "node" {
			t.Errorf("Command = %q, want %q", cfg.Command, "node")
		}
		if len(cfg.Args) != 1 || cfg.Args[0] != "a.js" {
			t.Errorf("Args = %v, want [a.js]", cfg.Args)
		}
		if cfg.Env["KEY"] != "val" {
			t.Errorf("Env[KEY] = %q, want %q", cfg.Env["KEY"], "val")
		}
	})

	t.Run("extracts HTTP server", func(t *testing.T) {
		cfg, err := ParseMCPServerConfig(tmpDir, "server-b")
		if err != nil {
			t.Fatalf("ParseMCPServerConfig: %v", err)
		}
		if cfg.URL != "https://b.example.com" {
			t.Errorf("URL = %q, want %q", cfg.URL, "https://b.example.com")
		}
		if cfg.Type != "streamable-http" {
			t.Errorf("Type = %q, want %q", cfg.Type, "streamable-http")
		}
	})

	t.Run("flat format fallback", func(t *testing.T) {
		flatDir := t.TempDir()
		os.WriteFile(filepath.Join(flatDir, "config.json"), []byte(`{
			"command": "npx",
			"args": ["-y", "@mcp/server"]
		}`), 0644)

		cfg, err := ParseMCPServerConfig(flatDir, "")
		if err != nil {
			t.Fatalf("ParseMCPServerConfig: %v", err)
		}
		if cfg.Command != "npx" {
			t.Errorf("Command = %q, want %q", cfg.Command, "npx")
		}
	})

	t.Run("missing server key returns flat fallback", func(t *testing.T) {
		// When serverKey doesn't match nested, falls back to flat parse.
		// For a nested config, flat parse returns empty since top-level has no command/url.
		cfg, err := ParseMCPServerConfig(tmpDir, "nonexistent")
		if err != nil {
			t.Fatalf("ParseMCPServerConfig: %v", err)
		}
		// Flat fallback on nested config yields empty MCPConfig (no command/url at top level).
		if cfg.Command != "" || cfg.URL != "" {
			t.Errorf("expected empty fallback, got command=%q url=%q", cfg.Command, cfg.URL)
		}
	})
}

func TestInstallMCP_PerServer_InvalidServerKey(t *testing.T) {
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "my-mcp")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{
		"mcpServers": {
			"real-server": {"command": "node"}
		}
	}`), 0644)

	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte("{}"), 0644)

	prov := provider.Provider{Slug: "test-provider"}
	origPath := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origPath }()

	item := catalog.ContentItem{
		Name:      "nonexistent",
		Type:      catalog.MCP,
		Path:      itemDir,
		ServerKey: "nonexistent",
	}
	_, err := installMCP(item, prov, tmpDir)
	if err == nil {
		t.Fatal("expected error for missing server key")
	}
}

// --- Cursor and Windsurf MCP install (syllago-14usr) -------------------
//
// Prior coverage in mcp_test.go exercised Zed, Cline, OpenCode, Kiro,
// Codex, and Amp install paths but nothing for Cursor or Windsurf — both
// of which declare MCP via JSONMergeSentinel in their provider records.
// The test-quality audit flagged this as an install gap: a broken MCP
// render for either provider would ship silently-wrong configs.
//
// These tests exercise:
//   - Installing a fresh canonical MCP into an empty settings file and
//     asserting the merged JSON via schema-level gjson paths (not
//     substring containment).
//   - The H2 conflict guard: a pre-existing MCP entry with the same name
//     that was NOT installed by syllago must cause installMCP to refuse
//     the install rather than silently clobber it.
//   - Syllago-managed overwrite: if installed.json records the same
//     server name as syllago-installed, installMCP must merge cleanly
//     over the existing entry.
//
// Note: Cursor and Windsurf are NOT currently handled by the
// mcpConfigPathImpl switch — production would fail with "MCP config path
// not defined". These tests use the existing mcpConfigPath test seam to
// validate the merge/render logic in isolation; filling in the real
// config-path mapping is tracked separately.

// TestInstallMCP_Cursor_MergesIntoMcpJsonSchema pins the canonical MCP →
// .cursor/mcp.json install path. Unlike the converter-level round-trip
// test, this exercises the full installer stack: it creates a fresh
// config.json item, runs installMCP with a Cursor provider, then asserts
// the result via gjson field paths so the test fails loudly if the merge
// produced an unexpected shape (missing mcpServers key, wrong indent,
// fields moved into a sibling object, etc.).
func TestInstallMCP_Cursor_MergesIntoMcpJsonSchema(t *testing.T) {
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "cursor-mcp")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	configJSON, err := json.Marshal(map[string]interface{}{
		"type":    "stdio",
		"command": "npx",
		"args":    []string{"-y", "@modelcontextprotocol/server-github"},
		"env":     map[string]string{"GITHUB_TOKEN": "placeholder"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "config.json"), configJSON, 0644); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(tmpDir, "cursor-mcp.json")
	if err := os.WriteFile(configFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{Name: "github", Type: catalog.MCP, Path: itemDir}
	prov := provider.Provider{Slug: "cursor", Name: "Cursor"}

	origPath := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origPath }()

	if _, err := installMCP(item, prov, tmpDir); err != nil {
		t.Fatalf("installMCP: %v", err)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}

	// Schema-level assertions — each path must exist with the expected
	// scalar/array value. A substring test would accept a degenerate
	// payload like `{"mcpServers":null,"github":{...}}`.
	got := gjson.GetBytes(data, "mcpServers.github")
	if !got.Exists() {
		t.Fatal("mcpServers.github not present after install")
	}
	if got := got.Get("command").String(); got != "npx" {
		t.Errorf("command = %q, want npx", got)
	}
	args := got.Get("args").Array()
	if len(args) != 2 || args[0].String() != "-y" || args[1].String() != "@modelcontextprotocol/server-github" {
		t.Errorf("args = %v, want [-y @modelcontextprotocol/server-github]", args)
	}
	if got := got.Get("env.GITHUB_TOKEN").String(); got != "placeholder" {
		t.Errorf("env.GITHUB_TOKEN = %q, want placeholder", got)
	}
	if got := got.Get("type").String(); got != "stdio" {
		t.Errorf("type = %q, want stdio (transport must survive merge)", got)
	}
}

// TestInstallMCP_Cursor_RefusesConflictWithUserEntry covers the H2 guard
// for Cursor. A pre-existing MCP server entry under the same name that
// the user added manually (not syllago-managed) must block the install
// rather than get silently overwritten — users rely on their own MCP
// entries surviving `syllago install`.
func TestInstallMCP_Cursor_RefusesConflictWithUserEntry(t *testing.T) {
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "cursor-mcp")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	configJSON, _ := json.Marshal(map[string]interface{}{
		"command": "npx",
		"args":    []string{"-y", "@modelcontextprotocol/server-github"},
	})
	os.WriteFile(filepath.Join(itemDir, "config.json"), configJSON, 0644)

	// User has a prior entry with the same key — not recorded in installed.json.
	configFile := filepath.Join(tmpDir, "cursor-mcp.json")
	existing := `{"mcpServers":{"github":{"command":"/usr/local/bin/custom-github","args":["--user-flag"]}}}`
	if err := os.WriteFile(configFile, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{Name: "github", Type: catalog.MCP, Path: itemDir}
	prov := provider.Provider{Slug: "cursor", Name: "Cursor"}

	origPath := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origPath }()

	_, err := installMCP(item, prov, tmpDir)
	if err == nil {
		t.Fatal("installMCP succeeded despite user-owned MCP entry — H2 guard failed for Cursor")
	}

	// The file must be untouched (aside from the backup the installer
	// writes before attempting the merge). gjson path should still return
	// the user's command, not syllago's.
	data, _ := os.ReadFile(configFile)
	if got := gjson.GetBytes(data, "mcpServers.github.command").String(); got != "/usr/local/bin/custom-github" {
		t.Errorf("user-owned command overwritten: got %q, want /usr/local/bin/custom-github", got)
	}
}

// TestInstallMCP_Cursor_AllowsOverwriteOfSyllagoManaged covers the other
// side of H2: when installed.json says syllago previously installed the
// same server name, the install path must overwrite cleanly. Without this
// path, a `syllago install` idempotency retry (or a registry update)
// would start failing with spurious "entry already exists" errors.
func TestInstallMCP_Cursor_AllowsOverwriteOfSyllagoManaged(t *testing.T) {
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "cursor-mcp")
	os.MkdirAll(itemDir, 0755)
	newConfig, _ := json.Marshal(map[string]interface{}{
		"command": "npx",
		"args":    []string{"-y", "@modelcontextprotocol/server-github", "--v2"},
	})
	os.WriteFile(filepath.Join(itemDir, "config.json"), newConfig, 0644)

	configFile := filepath.Join(tmpDir, "cursor-mcp.json")
	existing := `{"mcpServers":{"github":{"command":"npx","args":["-y","@modelcontextprotocol/server-github","--v1"]}}}`
	os.WriteFile(configFile, []byte(existing), 0644)

	inst := &Installed{
		MCP: []InstalledMCP{{
			Name:        "github",
			ServerNames: []string{"github"},
			Source:      "export",
		}},
	}
	if err := SaveInstalled(tmpDir, inst); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{Name: "github", Type: catalog.MCP, Path: itemDir}
	prov := provider.Provider{Slug: "cursor", Name: "Cursor"}

	origPath := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origPath }()

	if _, err := installMCP(item, prov, tmpDir); err != nil {
		t.Fatalf("installMCP refused overwrite of syllago-managed entry: %v", err)
	}

	data, _ := os.ReadFile(configFile)
	args := gjson.GetBytes(data, "mcpServers.github.args").Array()
	if len(args) != 3 || args[2].String() != "--v2" {
		t.Errorf("args = %v; expected the new --v2 array after overwrite", args)
	}
}

// TestInstallMCP_Windsurf_MergesIntoMcpConfigJsonSchema mirrors the
// Cursor happy-path test but targets Windsurf's .windsurf/mcp_config.json
// (note the different filename). Same schema-level assertions: every
// field must land on the expected JSON path rather than merely "appear
// somewhere in the output bytes".
func TestInstallMCP_Windsurf_MergesIntoMcpConfigJsonSchema(t *testing.T) {
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "windsurf-mcp")
	os.MkdirAll(itemDir, 0755)
	configJSON, _ := json.Marshal(map[string]interface{}{
		"command": "python",
		"args":    []string{"-m", "mcp_local"},
		"env":     map[string]string{"PORT": "3000"},
	})
	os.WriteFile(filepath.Join(itemDir, "config.json"), configJSON, 0644)

	configFile := filepath.Join(tmpDir, "windsurf-mcp_config.json")
	os.WriteFile(configFile, []byte("{}"), 0644)

	item := catalog.ContentItem{Name: "local-py", Type: catalog.MCP, Path: itemDir}
	prov := provider.Provider{Slug: "windsurf", Name: "Windsurf"}

	origPath := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origPath }()

	if _, err := installMCP(item, prov, tmpDir); err != nil {
		t.Fatalf("installMCP: %v", err)
	}

	data, _ := os.ReadFile(configFile)
	got := gjson.GetBytes(data, "mcpServers.local-py")
	if !got.Exists() {
		t.Fatal("mcpServers.local-py not present after Windsurf install")
	}
	if got := got.Get("command").String(); got != "python" {
		t.Errorf("command = %q, want python", got)
	}
	args := got.Get("args").Array()
	if len(args) != 2 || args[0].String() != "-m" || args[1].String() != "mcp_local" {
		t.Errorf("args = %v, want [-m mcp_local]", args)
	}
	if got := got.Get("env.PORT").String(); got != "3000" {
		t.Errorf("env.PORT = %q, want 3000", got)
	}
}

// TestInstallMCP_Windsurf_RefusesConflictWithUserEntry mirrors the Cursor
// H2 guard test against Windsurf's mcp_config.json shape. A user-owned
// entry must block the install.
func TestInstallMCP_Windsurf_RefusesConflictWithUserEntry(t *testing.T) {
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "windsurf-mcp")
	os.MkdirAll(itemDir, 0755)
	configJSON, _ := json.Marshal(map[string]interface{}{
		"command": "node",
		"args":    []string{"new.js"},
	})
	os.WriteFile(filepath.Join(itemDir, "config.json"), configJSON, 0644)

	configFile := filepath.Join(tmpDir, "windsurf-mcp_config.json")
	existing := `{"mcpServers":{"local-py":{"command":"python","args":["user-owned.py"]}}}`
	os.WriteFile(configFile, []byte(existing), 0644)

	item := catalog.ContentItem{Name: "local-py", Type: catalog.MCP, Path: itemDir}
	prov := provider.Provider{Slug: "windsurf", Name: "Windsurf"}

	origPath := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origPath }()

	_, err := installMCP(item, prov, tmpDir)
	if err == nil {
		t.Fatal("installMCP succeeded despite user-owned MCP entry — H2 guard failed for Windsurf")
	}

	data, _ := os.ReadFile(configFile)
	if got := gjson.GetBytes(data, "mcpServers.local-py.command").String(); got != "python" {
		t.Errorf("user-owned command overwritten: got %q, want python", got)
	}
}
