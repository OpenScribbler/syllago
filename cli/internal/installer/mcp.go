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

// ParseMCPServerConfig reads config.json and returns the MCPConfig for a specific server.
// For nested format, extracts the entry matching serverKey.
// For flat format (or empty serverKey), returns the single config.
func ParseMCPServerConfig(itemPath string, serverKey string) (*MCPConfig, error) {
	data, err := os.ReadFile(filepath.Join(itemPath, "config.json"))
	if err != nil {
		return nil, err
	}

	// If serverKey is provided, try nested format first.
	if serverKey != "" {
		result := gjson.GetBytes(data, "mcpServers."+serverKey)
		if result.Exists() {
			var cfg MCPConfig
			if err := json.Unmarshal([]byte(result.Raw), &cfg); err != nil {
				return nil, fmt.Errorf("parsing server %q: %w", serverKey, err)
			}
			return &cfg, nil
		}
	}

	// Fall back to flat format.
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
	case "cursor":
		// Cursor supports both ~/.cursor/mcp.json (global) and .cursor/mcp.json
		// (project) per https://cursor.com/docs/context/mcp. We install to the
		// project-local path for parity with the other per-repo providers
		// (copilot-cli, kiro, opencode, roo-code) and with cursor.go's
		// DiscoveryPaths, which treats the project file as the primary source.
		return filepath.Join(repoRoot, ".cursor", "mcp.json"), nil
	case "windsurf":
		// Windsurf only documents a global MCP config path; no project-local
		// alternative exists per https://docs.windsurf.com/windsurf/cascade/mcp.
		return filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"), nil
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

// MCPLocation describes one file where MCP configs were found.
type MCPLocation struct {
	Scope   SettingsScope
	Path    string
	JSONKey string // "mcpServers", "context_servers", "mcp", "amp.mcpServers", etc.
}

// FindMCPLocations returns all files where MCP configs exist for a provider.
// For providers that store MCP in settings.json (alongside hooks), it checks
// both global and project scopes. For providers with dedicated MCP files, it
// checks those. For Claude Code, it checks both settings.json files AND
// dedicated files (~/.claude.json, .mcp.json).
func FindMCPLocations(prov provider.Provider, projectRoot, baseDir string) []MCPLocation {
	jsonKey := MCPConfigKey(prov)
	if prov.Slug == "opencode" {
		jsonKey = "mcp"
	}
	if prov.Slug == "amp" {
		jsonKey = "amp.mcpServers"
	}

	var locs []MCPLocation

	// Check settings.json files (these can contain mcpServers for some providers).
	settingsLocs, _ := FindSettingsLocationsWithBase(prov, projectRoot, baseDir)
	for _, sl := range settingsLocs {
		data, err := os.ReadFile(sl.Path)
		if err != nil {
			continue
		}
		if gjson.GetBytes(data, jsonKey).Exists() {
			locs = append(locs, MCPLocation{
				Scope:   sl.Scope,
				Path:    sl.Path,
				JSONKey: jsonKey,
			})
		}
	}

	// Check dedicated MCP config files.
	cfgPath, err := mcpConfigPath(prov, projectRoot)
	if err == nil && cfgPath != "" {
		// Don't add duplicates (some providers use settings.json for both).
		dup := false
		for _, l := range locs {
			if l.Path == cfgPath {
				dup = true
				break
			}
		}
		if !dup {
			if _, err := os.Stat(cfgPath); err == nil {
				scope := ScopeGlobal
				// Heuristic: if the path is under projectRoot, it's project-scoped.
				if projectRoot != "" && isUnder(cfgPath, projectRoot) {
					scope = ScopeProject
				}
				locs = append(locs, MCPLocation{
					Scope:   scope,
					Path:    cfgPath,
					JSONKey: jsonKey,
				})
			}
		}
	}

	// Claude Code also has .mcp.json in the project root.
	if prov.Slug == "claude-code" && projectRoot != "" {
		mcpJSON := filepath.Join(projectRoot, ".mcp.json")
		dup := false
		for _, l := range locs {
			if l.Path == mcpJSON {
				dup = true
				break
			}
		}
		if !dup {
			if _, err := os.Stat(mcpJSON); err == nil {
				locs = append(locs, MCPLocation{
					Scope:   ScopeProject,
					Path:    mcpJSON,
					JSONKey: jsonKey,
				})
			}
		}
	}

	return locs
}

// isUnder reports whether path is inside dir.
func isUnder(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && len(rel) > 0 && rel[0] != '.'
}

// readMCPConfig reads and returns the JSON bytes from a provider's MCP config file.
// Strips JSONC comments for all providers — sjson requires valid JSON input.
// This permanently removes comments from settings files that use JSONC (e.g. Zed, OpenCode).
func readMCPConfig(cfgPath string, prov provider.Provider) ([]byte, error) {
	data, err := readJSONFile(cfgPath)
	if err != nil {
		return nil, err
	}
	data = converter.StripJSONCComments(data)
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

	// If item has a ServerKey, filter to just that one server.
	if item.ServerKey != "" {
		configData, ok := entries[item.ServerKey]
		if !ok {
			return "", fmt.Errorf("server %q not found in config.json", item.ServerKey)
		}
		entries = map[string]json.RawMessage{item.ServerKey: configData}
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

	// Load installed.json to check for syllago-managed entries
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return "", fmt.Errorf("loading installed.json: %w", err)
	}

	// Merge each server entry into the target config
	var serverNames []string
	for name, configData := range entries {
		if !catalog.IsValidItemName(name) {
			return "", fmt.Errorf("invalid MCP server name %q: names may only contain letters, numbers, hyphens, and underscores", name)
		}
		key := jsonKey + "." + name

		// H2: Check for collision with user-defined (non-syllago) server keys
		if gjson.GetBytes(fileData, key).Exists() {
			// Check if this key was installed by syllago (safe to overwrite)
			syllagoManaged := false
			for _, m := range inst.MCP {
				if m.ServerKey == name {
					syllagoManaged = true
					break
				}
				for _, sn := range m.ServerNames {
					if sn == name {
						syllagoManaged = true
						break
					}
				}
				if syllagoManaged {
					break
				}
			}
			if !syllagoManaged {
				return "", fmt.Errorf("MCP server %q already exists in %s and was not installed by syllago; use --force to overwrite", name, cfgPath)
			}
		}

		fileData, err = sjson.SetRawBytes(fileData, key, configData)
		if err != nil {
			return "", fmt.Errorf("setting %s: %w", key, err)
		}
		serverNames = append(serverNames, name)
	}

	if err := writeJSONFile(cfgPath, fileData); err != nil {
		return "", fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	// Record in installed.json (inst already loaded above for collision check)
	if item.ServerKey != "" {
		// Per-server install: one entry per server key
		inst.MCP = append(inst.MCP, InstalledMCP{
			Name:        item.Name,
			ServerKey:   item.ServerKey,
			Source:      "export",
			InstalledAt: time.Now(),
		})
	} else {
		// Legacy bulk install: track all server names together
		inst.MCP = append(inst.MCP, InstalledMCP{
			Name:        item.Name,
			ServerNames: serverNames,
			Source:      "export",
			InstalledAt: time.Now(),
		})
	}

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

	// Try per-server lookup first (new format), then legacy bulk lookup.
	instIdx := -1
	if item.ServerKey != "" {
		instIdx = inst.FindMCPByServerKey(item.Name, item.ServerKey)
	}
	if instIdx < 0 {
		instIdx = inst.FindMCP(item.Name)
	}
	if instIdx < 0 {
		return "", fmt.Errorf("%s was not installed by syllago", item.Name)
	}

	if err := backupFile(cfgPath); err != nil {
		return "", fmt.Errorf("backing up %s: %w", cfgPath, err)
	}

	// Determine which server keys to remove.
	var keysToRemove []string
	entry := inst.MCP[instIdx]
	if entry.ServerKey != "" {
		// Per-server entry: remove just this server key.
		keysToRemove = []string{entry.ServerKey}
	} else if len(entry.ServerNames) > 0 {
		// Legacy bulk entry: remove all tracked server names.
		keysToRemove = entry.ServerNames
	} else {
		// Very old legacy: item name was the server key.
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

	// Per-server check: if item has a ServerKey, check for that specific key.
	if item.ServerKey != "" {
		if gjson.GetBytes(fileData, jsonKey+"."+item.ServerKey).Exists() {
			return StatusInstalled
		}
		return StatusNotInstalled
	}

	// Legacy: check installed.json for bulk-installed entries.
	inst, err := LoadInstalled(repoRoot)
	if err == nil {
		idx := inst.FindMCP(item.Name)
		if idx >= 0 {
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

	// Fallback: check if item name exists as a server key
	if gjson.GetBytes(fileData, jsonKey+"."+item.Name).Exists() {
		return StatusInstalled
	}
	return StatusNotInstalled
}
