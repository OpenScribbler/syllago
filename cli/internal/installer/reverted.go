package installer

import (
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
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
		reverted[revertedKey(p)] = true
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

// revertedKey names a provider file the same way whichever path reaches
// it, such as through a symlink to the project directory. The revert may
// have deleted the file, so only its directory is resolved.
func revertedKey(p string) string {
	p = filepath.Clean(p)
	if dir, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		return filepath.Join(dir, filepath.Base(p))
	}
	return p
}

func hookReverted(h InstalledHook, reverted map[string]bool) bool {
	if h.GroupHash == "" {
		return false
	}
	return recordReverted(h.Provider, reverted, func(prov provider.Provider) (string, bool) {
		path, err := hookSettingsPath(prov)
		_, modelErr := hookStorageModelFor(prov.Slug)
		return path, err == nil && modelErr == nil && converter.AdapterFor(prov.Slug) != nil
	}, func(prov provider.Provider, path string) (bool, error) {
		model, _ := hookStorageModelFor(prov.Slug)
		existing, err := decodeExistingHooks(model, converter.AdapterFor(prov.Slug), path)
		if err != nil {
			return false, err
		}
		for _, eh := range existing {
			if hookIdentity(eh) == h.GroupHash {
				return true, nil
			}
		}
		if model != hookStorageSharedJSON {
			return false, nil
		}
		// v0.14.0 and earlier hashed the matcher group's JSON as written.
		data, err := readJSONFile(path)
		if err != nil {
			return false, err
		}
		held := false
		gjson.GetBytes(data, "hooks").ForEach(func(_, groups gjson.Result) bool {
			for _, g := range groups.Array() {
				held = held || computeGroupHash([]byte(g.Raw)) == h.GroupHash
			}
			return !held
		})
		return held, nil
	})
}

func mcpReverted(m InstalledMCP, repoRoot string, reverted map[string]bool) bool {
	return recordReverted(m.Provider, reverted, func(prov provider.Provider) (string, bool) {
		path, err := mcpConfigPath(prov, repoRoot)
		return path, err == nil
	}, func(prov provider.Provider, path string) (bool, error) {
		data, err := readMCPConfig(path, prov)
		if err != nil {
			return false, err
		}
		return mcpRecordInConfig(m, m.Name, data, MCPConfigKey(prov)), nil
	})
}

// recordReverted reports whether a record's hook or server is gone because
// the revert took it out. A record naming its provider goes when that
// provider's file is reverted and no longer holds it. One written before
// records named their provider could be any provider's, so it goes when
// some provider's file is reverted and no provider's file holds it. A file
// that cannot be read keeps the record.
func recordReverted(slug string, reverted map[string]bool,
	pathFor func(provider.Provider) (string, bool),
	holds func(provider.Provider, string) (bool, error)) bool {
	provs := provider.AllProviders
	if slug != "" {
		prov, ok := providerBySlug(slug)
		if !ok {
			return false
		}
		provs = []provider.Provider{prov}
	}
	touched := false
	for _, prov := range provs {
		path, ok := pathFor(prov)
		if !ok {
			continue
		}
		if reverted[revertedKey(path)] {
			touched = true
		}
		if held, err := holds(prov, path); err != nil || held {
			return false
		}
	}
	return touched
}

func providerBySlug(slug string) (provider.Provider, bool) {
	for _, p := range provider.AllProviders {
		if p.Slug == slug {
			return p, true
		}
	}
	return provider.Provider{}, false
}
