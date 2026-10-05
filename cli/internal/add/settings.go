package add

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

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
	// entry.
	ServerKey string
	Server    json.RawMessage
	Dest      string
}

// DiscoverSettings reads every settings file prov keeps hooks or MCP
// servers in (ct picks which) and returns one item per hook or server.
// Two hooks in one file that derive the same name are told apart with -2,
// -3, and so on. A settings file that cannot be read or parsed is left
// out and reported in unread.
//
// Items land under <type>/<provider>/<name> in the Library. When that
// directory holds a different item, or the same item added from another
// scope, the item takes the first -N directory that is free or holds it,
// so project and global copies sit side by side and a second add finds
// the first. Status is StatusInLibrary when Dest already holds the item.
// Dest and Status assume every discovered item is added; AddFromSettings
// places the items it is given afresh.
func DiscoverSettings(prov provider.Provider, projectRoot, baseDir, globalDir string, ct catalog.ContentType) (items []SettingsItem, unread []error, err error) {
	switch ct {
	case catalog.Hooks:
		locations, err := installer.FindSettingsLocationsWithBase(prov, projectRoot, baseDir)
		if err != nil {
			return nil, nil, fmt.Errorf("finding settings locations: %w", err)
		}
		for _, loc := range locations {
			data, err := os.ReadFile(loc.Path)
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					unread = append(unread, err)
				}
				continue
			}
			if !gjson.ValidBytes(data) {
				unread = append(unread, fmt.Errorf("parsing %s: invalid JSON", loc.Path))
				continue
			}
			// A settings file kept for other settings holds no hooks.
			if !gjson.GetBytes(data, "hooks").Exists() {
				continue
			}
			hooks, err := converter.SplitSettingsHooks(data, prov.Slug)
			if err != nil {
				unread = append(unread, fmt.Errorf("parsing %s: %w", loc.Path, err))
				continue
			}
			names := hookNames(hooks)
			for i, hook := range hooks {
				items = append(items, SettingsItem{
					DiscoveryItem: DiscoveryItem{Name: names[i], Type: catalog.Hooks, Path: loc.Path, Scope: loc.Scope.String()},
					Hook:          &hook,
				})
			}
		}
	case catalog.MCP:
		for _, loc := range installer.FindMCPLocations(prov, projectRoot, baseDir) {
			data, err := os.ReadFile(loc.Path)
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					unread = append(unread, err)
				}
				continue
			}
			if prov.Slug == "opencode" {
				data = converter.StripJSONCComments(data)
			}
			if !gjson.ValidBytes(data) {
				unread = append(unread, fmt.Errorf("parsing %s: invalid JSON", loc.Path))
				continue
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
				})
				return true
			})
		}
	default:
		return nil, nil, fmt.Errorf("content type %s is not kept in settings files", ct)
	}

	claimed := map[string]bool{}
	for i := range items {
		items[i].Dest, items[i].Status = settingsDest(globalDir, prov.Slug, &items[i], claimed)
		claimed[items[i].Dest] = true
	}
	return items, unread, nil
}

// hookNames names each hook of one settings file. A hook whose derived
// name an earlier hook took gets the first -N suffix no hook in the file
// derives or has taken, so a suffixed name never shadows a real one.
func hookNames(hooks []converter.HookData) []string {
	derived := make([]string, len(hooks))
	taken := map[string]bool{}
	for i, hook := range hooks {
		derived[i] = converter.DeriveHookName(hook)
	}
	natural := map[string]bool{}
	for _, name := range derived {
		natural[name] = true
	}
	names := make([]string, len(hooks))
	for i, name := range derived {
		if taken[name] {
			for n := 2; ; n++ {
				if candidate := fmt.Sprintf("%s-%d", name, n); !taken[candidate] && !natural[candidate] {
					name = candidate
					break
				}
			}
		}
		taken[name] = true
		names[i] = name
	}
	return names
}

// settingsDest picks the Library directory for item, skipping directories
// another item of this run has claimed. The item goes to the directory
// that holds it, wherever it sits, or else the first free one. A
// directory holds the item when its metadata records the item's scope
// and settings name. One added before names were recorded holds it at the
// base directory only, since only a real item of that name can sit at a
// -N directory without one.
func settingsDest(globalDir, provSlug string, item *SettingsItem, claimed map[string]bool) (string, ItemStatus) {
	base := filepath.Join(globalDir, string(item.Type), provSlug, item.Name)
	dir := func(i int) string {
		if i == 1 {
			return base
		}
		return fmt.Sprintf("%s-%d", base, i)
	}
	entries, _ := os.ReadDir(filepath.Dir(base))
	var held []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name() == item.Name {
			held = append(held, 1)
		} else if n, err := strconv.Atoi(strings.TrimPrefix(e.Name(), item.Name+"-")); err == nil && n > 1 && e.Name() == fmt.Sprintf("%s-%d", item.Name, n) {
			held = append(held, n)
		}
	}
	slices.Sort(held)
	for _, i := range held {
		if !claimed[dir(i)] && holdsItem(dir(i), i, item) {
			return dir(i), StatusInLibrary
		}
	}
	for i := 1; ; i++ {
		if _, err := os.Stat(dir(i)); err != nil && !claimed[dir(i)] {
			return dir(i), StatusNew
		}
	}
}

// holdsItem reports whether dest, the item's i-th directory, holds item.
func holdsItem(dest string, i int, item *SettingsItem) bool {
	meta, _ := metadata.Load(dest)
	if meta == nil || meta.SourceScope == "" {
		return i == 1
	}
	return meta.SourceScope == item.Scope && (meta.SourceName == item.Name || meta.SourceName == "" && i == 1)
}

// SettingsResult is the outcome of adding one settings item. LibraryName
// is the name the Library lists the item at Dest under, which install
// records and pins are keyed on. Bundled counts the hook scripts copied
// beside it; BundleErr is a script that could not be copied, which leaves
// the hook added.
type SettingsResult struct {
	AddResult
	Dest        string
	LibraryName string
	Bundled     int
	BundleErr   error
}

// AddFromSettings writes each item to the Library under globalDir: a hook
// as a hooks/0.1 manifest with the scripts its commands name, an MCP
// server as a config.json holding that server alone. Each item is placed
// as DiscoverSettings places it, counting only the items given here, so
// an item left out claims no directory. An item already in the Library is
// left as it is unless opts.Force. projectRoot names the project a
// project-scope item came from.
func AddFromSettings(items []SettingsItem, opts AddOptions, projectRoot, globalDir string) []SettingsResult {
	return AddFromSettingsAfter(nil, items, opts, projectRoot, globalDir)
}

// AddFromSettingsAfter adds items as AddFromSettings does, as the later
// part of an add whose earlier calls added placed. The placed items claim
// their directories first, so two items that share a scope and a name
// land side by side when they are added one call at a time.
func AddFromSettingsAfter(placed, items []SettingsItem, opts AddOptions, projectRoot, globalDir string) []SettingsResult {
	results := make([]SettingsResult, 0, len(items))
	claimed := map[string]bool{}
	for _, item := range placed {
		dest, _ := settingsDest(globalDir, opts.Provider, &item, claimed)
		claimed[dest] = true
	}
	for _, item := range items {
		item.Dest, item.Status = settingsDest(globalDir, opts.Provider, &item, claimed)
		claimed[item.Dest] = true
		results = append(results, addSettingsItem(item, opts, projectRoot, globalDir))
	}
	return results
}

func addSettingsItem(item SettingsItem, opts AddOptions, projectRoot, globalDir string) SettingsResult {
	r := SettingsResult{AddResult: AddResult{Name: item.Name, Type: item.Type}, Dest: item.Dest, LibraryName: filepath.Base(item.Dest)}
	// The Library lists a server under its key, whatever directory holds it.
	if item.Type == catalog.MCP {
		r.LibraryName = item.ServerKey
	}
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
		// Bundling rewrites each command's script path, so it works on a
		// copy that leaves the caller's item as discovered.
		hook := *item.Hook
		hook.Hooks = slices.Clone(hook.Hooks)
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
		// Every Library MCP config keeps its servers under mcpServers, the
		// key the catalog reads server names from, whichever key the
		// provider's settings file used.
		configJSON := fmt.Sprintf("{\n  \"mcpServers\": {\n    %q: %s\n  }\n}", item.ServerKey, item.Server)
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
		SourceName:       item.Name,
	}
	if opts.SourceRegistry != "" {
		meta.SourceType = "registry"
		meta.SourceSHA = opts.SourceSHA
	} else {
		// Laundering defense, as for any other add: a settings file reached
		// through a symlink into the Library, or one whose content matches
		// private Library content, keeps that content's registry.
		reg, vis := traceSymlinkTaint(item.Path, globalDir)
		if raw, err := os.ReadFile(item.Path); err == nil {
			// The hash lets a later add of the same file find this item.
			meta.SourceHash = sourceHash(raw)
			if reg == "" {
				reg, vis = hashMatchTaint(meta.SourceHash, globalDir)
			}
		}
		if reg != "" {
			meta.SourceRegistry, meta.SourceVisibility = reg, vis
		}
	}
	if item.Scope == "project" && projectRoot != "" {
		meta.SourceProject = filepath.Base(projectRoot)
	}
	if err := metadata.Save(item.Dest, meta); err != nil {
		return fail(fmt.Errorf("writing metadata: %w", err))
	}
	return r
}
