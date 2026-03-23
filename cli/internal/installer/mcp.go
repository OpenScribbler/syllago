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
		p := provider.ClineMCPSettingsPath()
		if p == "" {
			return "", fmt.Errorf("cannot determine Cline MCP settings path")
		}
		return p, nil
	case "roo-code":
		return filepath.Join(repoRoot, ".roo", "mcp.json"), nil
	}
	return "", fmt.Errorf("MCP config path not defined for %s", prov.Name)
}

// MCPConfigKey returns the JSON key under which MCP servers are stored.
// Most providers use "mcpServers"; Zed uses "context_servers".
func MCPConfigKey(prov provider.Provider) string {
	if prov.Slug == "zed" {
		return "context_servers"
	}
	return "mcpServers"
}

// MCPConfigPathFor returns the config file path where MCP servers are stored for the
// given provider. Exported for use by the loadout package.
func MCPConfigPathFor(prov provider.Provider, repoRoot string) (string, error) {
	return mcpConfigPath(prov, repoRoot)
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

// extractServerEntries reads config.json and returns a map of server names to
// their whitelisted config. Handles two formats:
//
//   - Nested: {"mcpServers": {"name": {...}}} — extracts each entry
//   - Flat: {"command": "node", ...} — uses itemName as the key
//
// ExtractServerEntries reads config.json and returns a map of server names to
// their whitelisted config. Handles two formats:
//
//   - Nested: {"mcpServers": {"name": {...}}} — extracts each entry
//   - Flat: {"command": "node", ...} — uses itemName as the key
func ExtractServerEntries(rawData []byte, itemName string, jsonKey string) (map[string]json.RawMessage, error) {
	entries := make(map[string]json.RawMessage)

	// Check for nested format: config.json wraps entries in the provider key
	wrapper := gjson.GetBytes(rawData, jsonKey)
	if wrapper.Exists() && wrapper.Type == gjson.JSON {
		// Nested format — extract each server entry
		wrapper.ForEach(func(key, value gjson.Result) bool {
			// Whitelist fields for each server entry
			var cfg MCPConfig
			if err := json.Unmarshal([]byte(value.Raw), &cfg); err != nil {
				return true // skip malformed entries
			}
			cleaned, err := json.Marshal(cfg)
			if err != nil {
				return true
			}
			entries[key.String()] = cleaned
			return true
		})
		if len(entries) > 0 {
			return entries, nil
		}
	}

	// Flat format — the entire config.json is a single server definition
	var cfg MCPConfig
	if err := json.Unmarshal(rawData, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config.json: %w", err)
	}
	cleaned, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("serializing config: %w", err)
	}
	entries[itemName] = cleaned
	return entries, nil
}

func installMCP(item catalog.ContentItem, prov provider.Provider, repoRoot string) (string, error) {
	// Read the MCP config from the content item
	rawData, err := os.ReadFile(filepath.Join(item.Path, "config.json"))
	if err != nil {
		return "", fmt.Errorf("reading config.json: %w", err)
	}

	jsonKey := MCPConfigKey(prov)

	// Extract server entries — handles both nested and flat config.json formats
	entries, err := ExtractServerEntries(rawData, item.Name, jsonKey)
	if err != nil {
		return "", err
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

	// Merge each server entry into the target config
	var serverNames []string
	for name, configData := range entries {
		if !catalog.IsValidItemName(name) {
			return "", fmt.Errorf("invalid MCP server name %q: names may only contain letters, numbers, hyphens, and underscores", name)
		}
		key := jsonKey + "." + name
		fileData, err = sjson.SetRawBytes(fileData, key, configData)
		if err != nil {
			return "", fmt.Errorf("setting %s: %w", key, err)
		}
		serverNames = append(serverNames, name)
	}

	if err := writeJSONFile(cfgPath, fileData); err != nil {
		return "", fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	// Record in installed.json — track each server name for uninstall
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return "", fmt.Errorf("loading installed.json: %w", err)
	}
	inst.MCP = append(inst.MCP, InstalledMCP{
		Name:        item.Name,
		ServerNames: serverNames,
		Source:      "export",
		InstalledAt: time.Now(),
	})
	if err := SaveInstalled(repoRoot, inst); err != nil {
		return "", fmt.Errorf("saving installed.json: %w", err)
	}

	return fmt.Sprintf("%s in %s", jsonKey, cfgPath), nil
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

	jsonKey := MCPConfigKey(prov)

	// Check installed.json for ownership
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return "", fmt.Errorf("loading installed.json: %w", err)
	}
	instIdx := inst.FindMCP(item.Name)
	if instIdx < 0 {
		return "", fmt.Errorf("%s was not installed by syllago", item.Name)
	}

	if err := backupFile(cfgPath); err != nil {
		return "", fmt.Errorf("backing up %s: %w", cfgPath, err)
	}

	// Determine which server keys to remove. New installs track ServerNames;
	// legacy installs (before this fix) used item.Name as the key.
	keysToRemove := inst.MCP[instIdx].ServerNames
	if len(keysToRemove) == 0 {
		keysToRemove = []string{item.Name}
	}

	for _, name := range keysToRemove {
		key := jsonKey + "." + name
		if gjson.GetBytes(fileData, key).Exists() {
			fileData, err = sjson.DeleteBytes(fileData, key)
			if err != nil {
				return "", fmt.Errorf("deleting %s: %w", key, err)
			}
		}
	}

	if err := writeJSONFile(cfgPath, fileData); err != nil {
		return "", fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	// Remove from installed.json
	inst.RemoveMCP(instIdx)
	if err := SaveInstalled(repoRoot, inst); err != nil {
		return "", fmt.Errorf("saving installed.json: %w", err)
	}

	return fmt.Sprintf("%s from %s", jsonKey, cfgPath), nil
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

	jsonKey := MCPConfigKey(prov)

	// Check installed.json first — it tracks which server names belong to this item
	inst, err := LoadInstalled(repoRoot)
	if err == nil {
		idx := inst.FindMCP(item.Name)
		if idx >= 0 {
			// Check that at least one tracked server key exists in config
			names := inst.MCP[idx].ServerNames
			if len(names) == 0 {
				names = []string{item.Name}
			}
			for _, name := range names {
				if gjson.GetBytes(fileData, jsonKey+"."+name).Exists() {
					return StatusInstalled
				}
			}
			return StatusNotInstalled
		}
	}

	// Fallback: check if item name exists as a server key (legacy or flat format)
	if gjson.GetBytes(fileData, jsonKey+"."+item.Name).Exists() {
		return StatusInstalled
	}
	return StatusNotInstalled
}
