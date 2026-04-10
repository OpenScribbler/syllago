package installer

import (
	"os"
	"path/filepath"

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

	// Build lookup sets for fast matching
	hookSet := make(map[string]map[string]bool) // groupHash -> true
	for _, h := range inst.Hooks {
		if h.GroupHash != "" {
			if hookSet[h.Event] == nil {
				hookSet[h.Event] = make(map[string]bool)
			}
			hookSet[h.Event][h.GroupHash] = true
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

		settingsPath := filepath.Join(home, prov.ConfigDir, "settings.json")
		data, readErr := os.ReadFile(settingsPath)
		if readErr != nil {
			continue // no settings file for this provider
		}

		// Check hooks: settings.json has hooks.<event> arrays
		hooksObj := gjson.GetBytes(data, "hooks")
		if hooksObj.Exists() && hooksObj.IsObject() {
			hooksObj.ForEach(func(event, eventArr gjson.Result) bool {
				if !eventArr.IsArray() {
					return true
				}
				eventName := event.String()
				for i, entry := range eventArr.Array() {
					entryHash := computeGroupHash([]byte(entry.Raw))
					if hookSet[eventName] != nil && hookSet[eventName][entryHash] {
						continue // tracked
					}
					orphans = append(orphans, OrphanEntry{
						Provider: prov.Slug,
						Type:     "hook",
						Key:      eventName,
						Index:    i,
					})
				}
				return true
			})
		}

		// Check MCP: settings.json has mcpServers.<name> objects
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
