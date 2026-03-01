package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
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
// Some providers store MCP config per-user (home dir), others per-project (repo root).
// Declared as a var so tests can override it.
var mcpConfigPath = mcpConfigPathImpl

func mcpConfigPathImpl(prov provider.Provider, repoRoot string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch prov.Slug {
	case "claude-code":
		return filepath.Join(home, ".claude.json"), nil
	case "gemini-cli":
		return filepath.Join(home, prov.ConfigDir, "settings.json"), nil
	case "copilot-cli":
		return filepath.Join(repoRoot, ".copilot", "mcp.json"), nil
	case "kiro":
		return filepath.Join(repoRoot, ".kiro", "settings", "mcp.json"), nil
	case "opencode":
		return filepath.Join(repoRoot, "opencode.json"), nil
	case "zed":
		return filepath.Join(home, ".config", "zed", "settings.json"), nil
	case "cline":
		return filepath.Join(repoRoot, ".vscode", "mcp.json"), nil
	case "roo-code":
		return filepath.Join(repoRoot, ".roo", "mcp.json"), nil
	}
	return "", fmt.Errorf("MCP config path not defined for %s", prov.Name)
}

// mcpConfigKey returns the JSON key under which MCP servers are stored.
// Most providers use "mcpServers"; Zed uses "context_servers".
func mcpConfigKey(prov provider.Provider) string {
	if prov.Slug == "zed" {
		return "context_servers"
	}
	return "mcpServers"
}

// readMCPConfig reads and returns the JSON bytes from a provider's MCP config file.
// For OpenCode, strips JSONC comments before returning.
func readMCPConfig(cfgPath string, prov provider.Provider) ([]byte, error) {
	data, err := readJSONFile(cfgPath)
	if err != nil {
		return nil, err
	}
	if prov.Slug == "opencode" {
		data = converter.StripJSONCComments(data)
	}
	return data, nil
}

func installMCP(item catalog.ContentItem, prov provider.Provider, repoRoot string) (string, error) {
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

	// Read target config file
	cfgPath, err := mcpConfigPath(prov, repoRoot)
	if err != nil {
		return "", err
	}

	if err := backupFile(cfgPath); err != nil {
		return "", fmt.Errorf("backing up %s: %w", cfgPath, err)
	}

	fileData, err := readMCPConfig(cfgPath, prov)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	// Set <key>.<name> to our config (key varies by provider)
	jsonKey := mcpConfigKey(prov)
	key := jsonKey + "." + item.Name
	fileData, err = sjson.SetRawBytes(fileData, key, configData)
	if err != nil {
		return "", fmt.Errorf("setting %s: %w", key, err)
	}

	if err := writeJSONFile(cfgPath, fileData); err != nil {
		return "", fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	// Record in installed.json
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return "", fmt.Errorf("loading installed.json: %w", err)
	}
	inst.MCP = append(inst.MCP, InstalledMCP{
		Name:        item.Name,
		Source:      "export",
		InstalledAt: time.Now(),
	})
	if err := SaveInstalled(repoRoot, inst); err != nil {
		return "", fmt.Errorf("saving installed.json: %w", err)
	}

	return fmt.Sprintf("%s.%s in %s", jsonKey, item.Name, cfgPath), nil
}

func uninstallMCP(item catalog.ContentItem, prov provider.Provider, repoRoot string) (string, error) {
	cfgPath, err := mcpConfigPath(prov, repoRoot)
	if err != nil {
		return "", err
	}

	fileData, err := readMCPConfig(cfgPath, prov)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	jsonKey := mcpConfigKey(prov)
	key := jsonKey + "." + item.Name

	// Check if entry exists in config file
	entry := gjson.GetBytes(fileData, key)
	if !entry.Exists() {
		return "", fmt.Errorf("%s not found in %s", key, cfgPath)
	}

	// Check installed.json for ownership
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return "", fmt.Errorf("loading installed.json: %w", err)
	}
	instIdx := inst.FindMCP(item.Name)
	if instIdx < 0 {
		return "", fmt.Errorf("%s was not installed by syllago", key)
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

	// Remove from installed.json
	inst.RemoveMCP(instIdx)
	if err := SaveInstalled(repoRoot, inst); err != nil {
		return "", fmt.Errorf("saving installed.json: %w", err)
	}

	return fmt.Sprintf("%s.%s from %s", jsonKey, item.Name, cfgPath), nil
}

func checkMCPStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string) Status {
	cfgPath, err := mcpConfigPath(prov, repoRoot)
	if err != nil {
		return StatusNotAvailable
	}

	fileData, err := readMCPConfig(cfgPath, prov)
	if err != nil {
		return StatusNotAvailable
	}

	jsonKey := mcpConfigKey(prov)
	key := jsonKey + "." + item.Name
	entry := gjson.GetBytes(fileData, key)
	if !entry.Exists() {
		return StatusNotInstalled
	}

	// Check installed.json for syllago ownership
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return StatusInstalled // entry exists in config, can't verify ownership
	}
	if inst.FindMCP(item.Name) >= 0 {
		return StatusInstalled // syllago-owned
	}
	// Entry exists in config but wasn't installed by syllago — still report installed
	return StatusInstalled
}
