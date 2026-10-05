package installer

import (
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// ForgetReverted drops the hook and MCP records whose provider file is one
// of paths and no longer holds what the record placed. Removing a loadout
// restores or deletes those files, which takes out anything installed into
// them after the apply; a record left behind would report it installed and
// block installing it again. Records for other files, and records whose
// file cannot be read, stay.
func ForgetReverted(inst *Installed, repoRoot string, paths []string) {
	reverted := make(map[string]bool, len(paths))
	for _, p := range paths {
		reverted[filepath.Clean(p)] = true
	}

	var hooks []InstalledHook
	for _, h := range inst.Hooks {
		if !hookReverted(h, reverted) {
			hooks = append(hooks, h)
		}
	}
	inst.Hooks = hooks

	var mcp []InstalledMCP
	for _, m := range inst.MCP {
		if !mcpReverted(m, repoRoot, reverted) {
			mcp = append(mcp, m)
		}
	}
	inst.MCP = mcp
}

func hookReverted(h InstalledHook, reverted map[string]bool) bool {
	prov, ok := providerBySlug(h.Provider)
	if !ok || h.GroupHash == "" {
		return false
	}
	path, err := hookSettingsPath(prov)
	if err != nil || !reverted[filepath.Clean(path)] {
		return false
	}
	adapter := converter.AdapterFor(prov.Slug)
	model, err := hookStorageModelFor(prov.Slug)
	if adapter == nil || err != nil {
		return false
	}
	existing, err := decodeExistingHooks(model, adapter, path)
	if err != nil {
		return false
	}
	for _, eh := range existing {
		if hookIdentity(eh) == h.GroupHash {
			return false
		}
	}
	return true
}

func mcpReverted(m InstalledMCP, repoRoot string, reverted map[string]bool) bool {
	prov, ok := providerBySlug(m.Provider)
	if !ok {
		return false
	}
	path, err := mcpConfigPath(prov, repoRoot)
	if err != nil || !reverted[filepath.Clean(path)] {
		return false
	}
	data, err := readMCPConfig(path, prov)
	if err != nil {
		return false
	}
	return !mcpRecordInConfig(m, m.Name, data, MCPConfigKey(prov))
}

func providerBySlug(slug string) (provider.Provider, bool) {
	for _, p := range provider.AllProviders {
		if p.Slug == slug {
			return p, true
		}
	}
	return provider.Provider{}, false
}
