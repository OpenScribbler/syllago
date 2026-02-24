package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
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
	mcpConfigPath = func(p provider.Provider) (string, error) {
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

	// Should have _nesco marker
	if !serverConfig.Get("_nesco").Bool() {
		t.Error("_nesco marker missing")
	}
}
