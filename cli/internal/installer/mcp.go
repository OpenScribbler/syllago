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
	case "devin":
		// Devin only documents a global MCP config path; no project-local
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

func installMCP(item catalog.ContentItem, prov provider.Provider, repoRoot string) (Placement, error) {
	// Read the MCP config from the content item
	rawData, err := os.ReadFile(filepath.Join(item.Path, "config.json"))
	if err != nil {
		return Placement{}, fmt.Errorf("reading config.json: %w", err)
	}

	jsonKey := MCPConfigKey(prov)

	// Extract server entries — handles both nested and flat config.json formats
	entries, err := ExtractServerEntries(rawData, item.Name, jsonKey)
	if err != nil {
		return Placement{}, err
	}

	// If item has a ServerKey, filter to just that one server.
	if item.ServerKey != "" {
		configData, ok := entries[item.ServerKey]
		if !ok {
			return Placement{}, fmt.Errorf("server %q not found in config.json", item.ServerKey)
		}
		entries = map[string]json.RawMessage{item.ServerKey: configData}
	}

	// Read target config file
	cfgPath, err := mcpConfigPath(prov, repoRoot)
	if err != nil {
		return Placement{}, err
	}

	fileData, err := readMCPConfig(cfgPath, prov)
	if err != nil {
		return Placement{}, fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	// Load installed.json to check for syllago-managed entries
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return Placement{}, fmt.Errorf("loading installed.json: %w", err)
	}
	if serverName, ok := legacyMCPInstalled(repoRoot, item, entries, prov.Slug, fileData, jsonKey); ok {
		return Placement{}, fmt.Errorf("MCP server %q already installed", serverName)
	}

	// Merge each server entry into the target config
	var serverNames []string
	var keys []string
	for name, configData := range entries {
		if !catalog.IsValidItemName(name) {
			return Placement{}, fmt.Errorf("invalid MCP server name %q: names may only contain letters, numbers, hyphens, and underscores", name)
		}
		key := jsonKey + "." + name

		// H2: Check for collision with user-defined (non-syllago) server keys
		if gjson.GetBytes(fileData, key).Exists() {
			// Check if this key was installed by syllago (safe to overwrite)
			syllagoManaged := mcpServerClaimed(inst, name, prov.Slug, true)
			if !syllagoManaged {
				return Placement{}, fmt.Errorf("MCP server %q already exists in %s and was not installed by syllago; use --force to overwrite", name, cfgPath)
			}
		}

		fileData, err = sjson.SetRawBytes(fileData, key, configData)
		if err != nil {
			return Placement{}, fmt.Errorf("setting %s: %w", key, err)
		}
		serverNames = append(serverNames, name)
		keys = append(keys, key)
	}

	if err := backupFile(cfgPath); err != nil {
		return Placement{}, fmt.Errorf("backing up %s: %w", cfgPath, err)
	}

	if err := writeJSONFile(cfgPath, fileData); err != nil {
		return Placement{}, fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	// Record in installed.json (inst already loaded above for collision check)
	if item.ServerKey != "" {
		// Per-server install: one entry per server key
		inst.MCP = append(inst.MCP, InstalledMCP{
			Name:        item.Name,
			ServerKey:   item.ServerKey,
			Source:      "export",
			Provider:    prov.Slug,
			InstalledAt: time.Now(),
		})
	} else {
		// Legacy bulk install: track all server names together
		inst.MCP = append(inst.MCP, InstalledMCP{
			Name:        item.Name,
			ServerNames: serverNames,
			Source:      "export",
			Provider:    prov.Slug,
			InstalledAt: time.Now(),
		})
	}

	if err := SaveInstalled(repoRoot, inst); err != nil {
		return Placement{}, fmt.Errorf("saving installed.json: %w", err)
	}

	desc := fmt.Sprintf("%s in %s", jsonKey, cfgPath)
	return Placement{
		Mechanism: MechanismMCPMerge,
		Path:      cfgPath,
		Keys:      keys,
		desc:      desc,
	}, nil
}

func uninstallMCP(item catalog.ContentItem, prov provider.Provider, repoRoot string) (Placement, error) {
	return uninstallMCPAtRoot(item, prov, repoRoot, true)
}

func uninstallMCPAtRoot(item catalog.ContentItem, prov provider.Provider, repoRoot string, allowLegacyFallback bool) (Placement, error) {
	cfgPath, err := mcpConfigPath(prov, repoRoot)
	if err != nil {
		return Placement{}, err
	}

	fileData, err := readMCPConfig(cfgPath, prov)
	if err != nil {
		return Placement{}, fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	jsonKey := MCPConfigKey(prov)

	// Check installed.json for ownership
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return Placement{}, fmt.Errorf("loading installed.json: %w", err)
	}

	// Try per-server lookup first (new format), then legacy bulk lookup.
	instIdx := findMCPInstallRecord(inst, item, prov.Slug)
	if instIdx < 0 {
		if allowLegacyFallback {
			if legacyRoot := legacyRootWithMCPRecord(repoRoot, item, prov.Slug); legacyRoot != "" {
				return uninstallMCPAtRoot(item, prov, legacyRoot, false)
			}
		}
		return Placement{}, fmt.Errorf("%s was not installed by syllago", item.Name)
	}
	// An entry with no provider may belong to another provider. Remove it
	// only when this provider's config holds its server, as hooks require a
	// matching hook in the target settings.
	if entry := inst.MCP[instIdx]; entry.Provider == "" && !mcpRecordInConfig(entry, item.Name, fileData, jsonKey) {
		return Placement{}, fmt.Errorf("%s was not installed by syllago for %s", item.Name, prov.Name)
	}

	if err := backupFile(cfgPath); err != nil {
		return Placement{}, fmt.Errorf("backing up %s: %w", cfgPath, err)
	}

	var keys []string
	for _, name := range inst.MCP[instIdx].serverKeys(item.Name) {
		key := jsonKey + "." + name
		keys = append(keys, key)
		if gjson.GetBytes(fileData, key).Exists() {
			fileData, err = sjson.DeleteBytes(fileData, key)
			if err != nil {
				return Placement{}, fmt.Errorf("deleting %s: %w", key, err)
			}
		}
	}

	if err := writeJSONFile(cfgPath, fileData); err != nil {
		return Placement{}, fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	// Remove from installed.json
	inst.RemoveMCP(instIdx)
	if err := SaveInstalled(repoRoot, inst); err != nil {
		return Placement{}, fmt.Errorf("saving installed.json: %w", err)
	}

	desc := fmt.Sprintf("%s from %s", jsonKey, cfgPath)
	return Placement{
		Mechanism: MechanismMCPMerge,
		Path:      cfgPath,
		Keys:      keys,
		desc:      desc,
	}, nil
}

// mcpStatus is checkMCPStatus with the error that left its status a guess,
// as StatusOf describes.
func mcpStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string) (Status, error) {
	status, err := mcpStatusAtRoot(item, prov, repoRoot)
	if status != StatusNotInstalled {
		return status, err
	}
	if legacyRoot := legacyInstalledRoot(repoRoot); legacyRoot != "" {
		legacyStatus, legacyErr := mcpStatusAtRoot(item, prov, legacyRoot)
		if legacyStatus == StatusInstalled {
			return StatusInstalled, nil
		}
		if err == nil {
			err = legacyErr
		}
	}
	return status, err
}

func checkMCPStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string) Status {
	status, _ := mcpStatus(item, prov, repoRoot)
	return status
}

func mcpStatusAtRoot(item catalog.ContentItem, prov provider.Provider, repoRoot string) (Status, error) {
	cfgPath, err := mcpConfigPath(prov, repoRoot)
	if err != nil {
		return StatusNotAvailable, nil
	}

	fileData, err := readMCPConfig(cfgPath, prov)
	if err != nil {
		return StatusNotAvailable, err
	}

	jsonKey := MCPConfigKey(prov)

	// Per-server check: if item has a ServerKey, check for that specific key.
	if item.ServerKey != "" {
		if gjson.GetBytes(fileData, jsonKey+"."+item.ServerKey).Exists() {
			return StatusInstalled, nil
		}
		return StatusNotInstalled, nil
	}

	// Legacy: check installed.json for bulk-installed entries. When it
	// cannot be read, a server it names under another key goes unseen.
	inst, instErr := LoadInstalled(repoRoot)
	if instErr == nil {
		idx := inst.FindMCP(item.Name, prov.Slug)
		if idx >= 0 {
			names := inst.MCP[idx].ServerNames
			if len(names) == 0 {
				names = []string{item.Name}
			}
			for _, name := range names {
				if gjson.GetBytes(fileData, jsonKey+"."+name).Exists() {
					return StatusInstalled, nil
				}
			}
			return StatusNotInstalled, nil
		}
	}

	// Fallback: check if item name exists as a server key
	if gjson.GetBytes(fileData, jsonKey+"."+item.Name).Exists() {
		return StatusInstalled, nil
	}
	return StatusNotInstalled, instErr
}

// legacyMCPInstalled reports whether the legacy root's installed.json
// records item, or one of its servers, as installed on the provider with
// slug provSlug. fileData is that provider's MCP config and jsonKey its
// servers key. An entry with no provider counts only when its server is
// already in fileData, as hookTracked explains for hooks.
func legacyMCPInstalled(repoRoot string, item catalog.ContentItem, entries map[string]json.RawMessage, provSlug string, fileData []byte, jsonKey string) (string, bool) {
	legacyRoot := legacyInstalledRoot(repoRoot)
	if legacyRoot == "" {
		return "", false
	}
	inst, err := LoadInstalled(legacyRoot)
	if err != nil {
		return "", false
	}
	if idx := findMCPInstallRecord(inst, item, provSlug); idx >= 0 {
		entry := inst.MCP[idx]
		if entry.Provider != "" || mcpRecordInConfig(entry, item.Name, fileData, jsonKey) {
			return item.Name, true
		}
	}
	for name := range entries {
		if mcpServerClaimed(inst, name, provSlug, gjson.GetBytes(fileData, jsonKey+"."+name).Exists()) {
			return name, true
		}
	}
	return "", false
}

// mcpRecordInConfig reports whether any server the entry placed is in
// fileData under jsonKey.
func mcpRecordInConfig(entry InstalledMCP, itemName string, fileData []byte, jsonKey string) bool {
	for _, key := range entry.serverKeys(itemName) {
		if gjson.GetBytes(fileData, jsonKey+"."+key).Exists() {
			return true
		}
	}
	return false
}

func legacyRootWithMCPRecord(repoRoot string, item catalog.ContentItem, provSlug string) string {
	legacyRoot := legacyInstalledRoot(repoRoot)
	if legacyRoot == "" {
		return ""
	}
	inst, err := LoadInstalled(legacyRoot)
	if err != nil {
		return ""
	}
	if findMCPInstallRecord(inst, item, provSlug) < 0 {
		return ""
	}
	return legacyRoot
}

// findMCPInstallRecord prefers a record for this provider, whether it is a
// per-server record (installer) or a bulk one (loadout apply), over a record
// with no provider.
func findMCPInstallRecord(inst *Installed, item catalog.ContentItem, provSlug string) int {
	byName := inst.FindMCP(item.Name, provSlug)
	if item.ServerKey == "" {
		return byName
	}
	byKey := inst.FindMCPByServerKey(item.Name, item.ServerKey, provSlug)
	if byKey >= 0 && (inst.MCP[byKey].Provider == provSlug || byName < 0 || inst.MCP[byName].Provider != provSlug) {
		return byKey
	}
	return byName
}

// mcpServerClaimed reports whether an entry in inst placed serverName on the
// provider with slug provSlug. An entry with no provider claims the server
// only when inTarget says the provider's config holds it.
func mcpServerClaimed(inst *Installed, serverName, provSlug string, inTarget bool) bool {
	for _, m := range inst.MCP {
		if m.Provider != provSlug && (m.Provider != "" || !inTarget) {
			continue
		}
		if m.ServerKey == serverName {
			return true
		}
		for _, sn := range m.ServerNames {
			if sn == serverName {
				return true
			}
		}
	}
	return false
}
