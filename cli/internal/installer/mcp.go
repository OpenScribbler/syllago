package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/provider"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// MCPConfig represents a parsed MCP server configuration.
type MCPConfig struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// ParseMCPConfig reads and parses config.json from an MCP item directory.
func ParseMCPConfig(itemPath string) (*MCPConfig, error) {
	data, err := os.ReadFile(filepath.Join(itemPath, "config.json"))
	if err != nil {
		return nil, err
	}
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config.json: %w", err)
	}
	return &cfg, nil
}

// CheckEnvVars returns a map of env var name -> whether it's currently set.
func CheckEnvVars(cfg *MCPConfig) map[string]bool {
	result := make(map[string]bool)
	for k := range cfg.Env {
		_, set := os.LookupEnv(k)
		result[k] = set
	}
	return result
}

// mcpConfigPath returns the config file path where MCP servers are stored for the given provider.
// Claude Code: ~/.claude.json (root-level dotfile)
// Gemini CLI: ~/.gemini/settings.json (inside config dir)
// Declared as a var so tests can override it.
var mcpConfigPath = mcpConfigPathImpl

func mcpConfigPathImpl(prov provider.Provider) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch prov.Slug {
	case "claude-code":
		return filepath.Join(home, ".claude.json"), nil
	case "gemini-cli":
		return filepath.Join(home, prov.ConfigDir, "settings.json"), nil
	}
	return "", fmt.Errorf("MCP config path not defined for %s", prov.Name)
}

func installMCP(item catalog.ContentItem, prov provider.Provider, _ string) (string, error) {
	// Read the MCP config from the content item
	rawData, err := os.ReadFile(filepath.Join(item.Path, "config.json"))
	if err != nil {
		return "", fmt.Errorf("reading config.json: %w", err)
	}

	// Parse into struct to whitelist fields — unknown fields are dropped
	var cfg MCPConfig
	if err := json.Unmarshal(rawData, &cfg); err != nil {
		return "", fmt.Errorf("parsing config.json: %w", err)
	}

	// Re-serialize to produce only whitelisted fields
	configData, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("serializing config: %w", err)
	}

	// Add _nesco marker
	configData, err = sjson.SetBytes(configData, "_nesco", true)
	if err != nil {
		return "", fmt.Errorf("adding marker: %w", err)
	}

	// Read target config file
	cfgPath, err := mcpConfigPath(prov)
	if err != nil {
		return "", err
	}

	if err := backupFile(cfgPath); err != nil {
		return "", fmt.Errorf("backing up %s: %w", cfgPath, err)
	}

	fileData, err := readJSONFile(cfgPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	// Set mcpServers.<name> to our config
	key := "mcpServers." + item.Name
	fileData, err = sjson.SetRawBytes(fileData, key, configData)
	if err != nil {
		return "", fmt.Errorf("setting %s: %w", key, err)
	}

	if err := writeJSONFile(cfgPath, fileData); err != nil {
		return "", fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	return fmt.Sprintf("mcpServers.%s in %s", item.Name, cfgPath), nil
}

func uninstallMCP(item catalog.ContentItem, prov provider.Provider, _ string) (string, error) {
	cfgPath, err := mcpConfigPath(prov)
	if err != nil {
		return "", err
	}

	fileData, err := readJSONFile(cfgPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	key := "mcpServers." + item.Name

	// Check if it exists and has _nesco marker
	entry := gjson.GetBytes(fileData, key)
	if !entry.Exists() {
		return "", fmt.Errorf("%s not found in %s", key, cfgPath)
	}
	if !gjson.GetBytes(fileData, key+"._nesco").Bool() {
		return "", fmt.Errorf("%s was not installed by nesco", key)
	}

	if err := backupFile(cfgPath); err != nil {
		return "", fmt.Errorf("backing up %s: %w", cfgPath, err)
	}

	fileData, err = sjson.DeleteBytes(fileData, key)
	if err != nil {
		return "", fmt.Errorf("deleting %s: %w", key, err)
	}

	if err := writeJSONFile(cfgPath, fileData); err != nil {
		return "", fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	return fmt.Sprintf("mcpServers.%s from %s", item.Name, cfgPath), nil
}

func checkMCPStatus(item catalog.ContentItem, prov provider.Provider, _ string) Status {
	cfgPath, err := mcpConfigPath(prov)
	if err != nil {
		return StatusNotAvailable
	}

	fileData, err := readJSONFile(cfgPath)
	if err != nil {
		return StatusNotAvailable
	}

	key := "mcpServers." + item.Name
	entry := gjson.GetBytes(fileData, key)
	if !entry.Exists() {
		return StatusNotInstalled
	}
	if gjson.GetBytes(fileData, key+"._nesco").Bool() {
		return StatusInstalled
	}
	// Entry exists but wasn't installed by nesco -- treat as installed
	// (could be manually added -- don't overwrite)
	return StatusInstalled
}
