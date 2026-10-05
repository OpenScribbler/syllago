package add

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/gjson"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// SettingsItem is one hook or MCP server read out of a provider's settings
// file. Path is the settings file, Scope the scope it was found in, and
// Dest the Library directory an add writes it to.
type SettingsItem struct {
	DiscoveryItem
	// Hook is the single hook a hooks item holds.
	Hook *converter.HookData
	// ServerKey and Server are an MCP item's server name and its raw JSON
	// entry; JSONKey is the settings key the servers sit under.
	ServerKey string
	Server    json.RawMessage
	JSONKey   string
	Dest      string
}

// DiscoverSettings reads every settings file prov keeps hooks or MCP
// servers in (ct picks which) and returns one item per hook or server.
// Two hooks in one file that derive the same name are told apart with -2,
// -3, and so on.
//
// Items land under <type>/<provider>/<name> in the Library. When that
// directory holds an item added from another scope, the item takes the
// first -N directory that is free or holds an item of its own scope, so
// project and global copies sit side by side and a second add finds the
// first. Status is StatusInLibrary when Dest already holds the item.
func DiscoverSettings(prov provider.Provider, projectRoot, baseDir, globalDir string, ct catalog.ContentType) ([]SettingsItem, error) {
	var items []SettingsItem
	switch ct {
	case catalog.Hooks:
		locations, err := installer.FindSettingsLocationsWithBase(prov, projectRoot, baseDir)
		if err != nil {
			return nil, fmt.Errorf("finding settings locations: %w", err)
		}
		for _, loc := range locations {
			data, err := os.ReadFile(loc.Path)
			if err != nil {
				continue
			}
			hooks, err := converter.SplitSettingsHooks(data, prov.Slug)
			if err != nil {
				continue
			}
			seen := map[string]int{}
			for _, hook := range hooks {
				name := converter.DeriveHookName(hook)
				seen[name]++
				if n := seen[name]; n > 1 {
					name = fmt.Sprintf("%s-%d", name, n)
				}
				items = append(items, SettingsItem{
					DiscoveryItem: DiscoveryItem{Name: name, Type: catalog.Hooks, Path: loc.Path, Scope: loc.Scope.String()},
					Hook:          &hook,
				})
			}
		}
	case catalog.MCP:
		for _, loc := range installer.FindMCPLocations(prov, projectRoot, baseDir) {
			data, err := os.ReadFile(loc.Path)
			if err != nil {
				continue
			}
			if prov.Slug == "opencode" {
				data = converter.StripJSONCComments(data)
			}
			servers := gjson.GetBytes(data, loc.JSONKey)
			if !servers.Exists() || servers.Type != gjson.JSON {
				continue
			}
			servers.ForEach(func(key, value gjson.Result) bool {
				items = append(items, SettingsItem{
					DiscoveryItem: DiscoveryItem{Name: key.String(), Type: catalog.MCP, Path: loc.Path, Scope: loc.Scope.String()},
					ServerKey:     key.String(),
					Server:        json.RawMessage(value.Raw),
					JSONKey:       loc.JSONKey,
				})
				return true
			})
		}
	default:
		return nil, fmt.Errorf("content type %s is not kept in settings files", ct)
	}

	claimed := map[string]bool{}
	for i := range items {
		items[i].Dest, items[i].Status = settingsDest(globalDir, prov.Slug, &items[i], claimed)
		claimed[items[i].Dest] = true
	}
	return items, nil
}

// settingsDest picks the Library directory for item, skipping directories
// another item of this discovery has claimed.
func settingsDest(globalDir, provSlug string, item *SettingsItem, claimed map[string]bool) (string, ItemStatus) {
	base := filepath.Join(globalDir, string(item.Type), provSlug, item.Name)
	for i := 1; ; i++ {
		dest := base
		if i > 1 {
			dest = fmt.Sprintf("%s-%d", base, i)
		}
		if claimed[dest] {
			continue
		}
		info, err := os.Stat(dest)
		if err != nil || !info.IsDir() {
			return dest, StatusNew
		}
		meta, _ := metadata.Load(dest)
		if meta == nil || meta.SourceScope == "" || meta.SourceScope == item.Scope {
			return dest, StatusInLibrary
		}
	}
}

// SettingsResult is the outcome of adding one settings item. Bundled
// counts the hook scripts copied beside it; BundleErr is a script that
// could not be copied, which leaves the hook added.
type SettingsResult struct {
	AddResult
	Dest      string
	Bundled   int
	BundleErr error
}

// AddFromSettings writes each item to its Dest: a hook as a hooks/0.1
// manifest with the scripts its commands name, an MCP server as a
// config.json holding that server alone. An item already in the Library
// is left as it is unless opts.Force. projectRoot names the project a
// project-scope item came from.
func AddFromSettings(items []SettingsItem, opts AddOptions, projectRoot string) []SettingsResult {
	results := make([]SettingsResult, 0, len(items))
	for _, item := range items {
		results = append(results, addSettingsItem(item, opts, projectRoot))
	}
	return results
}

func addSettingsItem(item SettingsItem, opts AddOptions, projectRoot string) SettingsResult {
	r := SettingsResult{AddResult: AddResult{Name: item.Name, Type: item.Type}, Dest: item.Dest}
	fail := func(err error) SettingsResult {
		r.Status, r.Error = AddStatusError, err
		return r
	}
	if item.Status == StatusInLibrary && !opts.Force {
		r.Status = AddStatusUpToDate
		return r
	}
	r.Status = AddStatusAdded
	if item.Status != StatusNew {
		r.Status = AddStatusUpdated
	}
	if opts.DryRun {
		return r
	}
	if err := os.MkdirAll(item.Dest, 0o755); err != nil {
		return fail(fmt.Errorf("creating %s: %w", item.Dest, err))
	}

	metaName := item.Name
	var bundledMeta []metadata.BundledScriptMeta
	switch item.Type {
	case catalog.Hooks:
		metaName = filepath.Base(item.Dest)
		hook := *item.Hook
		bundled, err := converter.BundleHookScripts(&hook, filepath.Dir(item.Path), item.Dest)
		r.Bundled, r.BundleErr = len(bundled), err
		for _, b := range bundled {
			bundledMeta = append(bundledMeta, metadata.BundledScriptMeta{OriginalPath: b.OriginalPath, Filename: b.Filename})
		}
		manifest, err := converter.ManifestFromHookData(hook)
		if err != nil {
			return fail(fmt.Errorf("building manifest: %w", err))
		}
		hookJSON, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return fail(fmt.Errorf("marshaling hook: %w", err))
		}
		if err := os.WriteFile(filepath.Join(item.Dest, "hook.json"), hookJSON, 0o644); err != nil {
			return fail(fmt.Errorf("writing hook.json: %w", err))
		}
	case catalog.MCP:
		configJSON := fmt.Sprintf("{\n  %q: {\n    %q: %s\n  }\n}", item.JSONKey, item.ServerKey, item.Server)
		if err := os.WriteFile(filepath.Join(item.Dest, "config.json"), []byte(configJSON), 0o644); err != nil {
			return fail(fmt.Errorf("writing config.json: %w", err))
		}
	}

	if item.DisplayName != "" {
		metaName = item.DisplayName
	}
	now := timeNow()
	meta := &metadata.Meta{
		ID:               metadata.NewID(),
		Name:             metaName,
		Description:      item.Description,
		Type:             string(item.Type),
		BundledScripts:   bundledMeta,
		AddedAt:          &now,
		SourceProvider:   opts.Provider,
		SourceFormat:     "json",
		SourceType:       "provider",
		SourceRegistry:   opts.SourceRegistry,
		SourceVisibility: opts.SourceVisibility,
		SourceScope:      item.Scope,
	}
	if opts.SourceRegistry != "" {
		meta.SourceType = "registry"
		meta.SourceSHA = opts.SourceSHA
	}
	if item.Scope == "project" && projectRoot != "" {
		meta.SourceProject = filepath.Base(projectRoot)
	}
	if err := metadata.Save(item.Dest, meta); err != nil {
		return fail(fmt.Errorf("writing metadata: %w", err))
	}
	return r
}
