package installer

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
)

// OrphanEntry describes a hook or MCP entry found in a provider's settings.json
// that is not tracked by installed.json. These entries may have been left behind
// by a crash between writing settings.json and updating installed.json.
type OrphanEntry struct {
	Provider string `json:"provider"`
	Type     string `json:"type"`  // "hook" or "mcp"
	Key      string `json:"key"`   // event name for hooks, server name for MCP
	Index    int    `json:"index"` // array index for hooks, -1 for MCP
}

// CheckOrphanedMerges reads settings.json for each detected provider and
// compares hooks/MCP entries against installed.json. Returns entries present
// in settings but not tracked by installed.json.
func CheckOrphanedMerges(projectRoot string, providers []provider.Provider) ([]OrphanEntry, error) {
	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		return nil, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Build lookup sets for fast matching. Hooks are matched by their canonical
	// identity (installed.json GroupHash is the post-round-trip
	// sha256(event|matcher|command|name)), the same identity install computes.
	trackedHooks := make(map[string]bool)
	for _, h := range inst.Hooks {
		if h.GroupHash != "" {
			trackedHooks[h.GroupHash] = true
		}
	}
	mcpSet := make(map[string]bool)
	for _, m := range inst.MCP {
		if m.ServerKey != "" {
			mcpSet[m.ServerKey] = true
		}
		for _, name := range m.ServerNames {
			mcpSet[name] = true
		}
	}

	var orphans []OrphanEntry

	for _, prov := range providers {
		if !prov.Detected || prov.ConfigDir == "" {
			continue
		}

		// Hooks: decode the provider's real hook file via its adapter and match
		// each decoded hook by canonical identity. Providers with no adapter or
		// no supported hook path (Phase 1b) are skipped so they are never
		// false-flagged.
		if adapter := converter.AdapterFor(prov.Slug); adapter != nil {
			if hookPath, pErr := HookConfigPath(prov, home); pErr == nil {
				if data, readErr := os.ReadFile(hookPath); readErr == nil {
					if decoded, dErr := adapter.Decode(data); dErr == nil {
						for i, hk := range decoded.Hooks {
							if trackedHooks[hookIdentity(hk)] {
								continue // tracked by syllago
							}
							orphans = append(orphans, OrphanEntry{
								Provider: prov.Slug,
								Type:     "hook",
								Key:      nativeEventFor(hk.Event, prov.Slug),
								Index:    i,
							})
						}
					}
				}
			}
		}

		// MCP: unchanged — mcpServers.<name> objects in <ConfigDir>/settings.json.
		settingsPath := filepath.Join(home, prov.ConfigDir, "settings.json")
		data, readErr := os.ReadFile(settingsPath)
		if readErr != nil {
			continue // no settings file for this provider
		}
		mcpObj := gjson.GetBytes(data, "mcpServers")
		if mcpObj.Exists() && mcpObj.IsObject() {
			mcpObj.ForEach(func(key, _ gjson.Result) bool {
				serverName := key.String()
				if !mcpSet[serverName] {
					orphans = append(orphans, OrphanEntry{
						Provider: prov.Slug,
						Type:     "mcp",
						Key:      serverName,
						Index:    -1,
					})
				}
				return true
			})
		}
	}

	return orphans, nil
}
