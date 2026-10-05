package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/gjson"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// discoverSettingsFromProvider reads prov's settings files for the hooks or
// MCP servers (ct picks which) and returns one discovery item per entry,
// placed and annotated with library status by add.DiscoverSettings. A
// settings file or entry that could not be read is returned in unread.
func discoverSettingsFromProvider(prov provider.Provider, projectRoot, baseDir, contentRoot string, ct catalog.ContentType) ([]addDiscoveryItem, []error) {
	settings, unread, err := add.DiscoverSettings(prov, projectRoot, baseDir, contentRoot, ct)
	if err != nil {
		return nil, []error{err}
	}
	homeDir, _ := os.UserHomeDir()
	return settingsItemsToDiscovery(settings, prov.Slug, func(path string) string {
		return providerRelPath(path, "", homeDir)
	}), unread
}

// discoverSettingsFromFolder reads the hooks and MCP servers (typeSet picks
// which) in the provider settings files found in dir: those native names,
// then those the analyzer confirm items name under a known provider. It
// returns one discovery item per entry, and covered, the files it read
// entries from, keyed by type and path relative to dir.
func discoverSettingsFromFolder(dir string, native catalog.NativeScanResult, confirm []*analyzer.DetectedItem, typeSet map[catalog.ContentType]bool, contentRoot string) (items []addDiscoveryItem, covered map[string]bool, unread []error) {
	type fileGroup struct {
		prov  provider.Provider
		ct    catalog.ContentType
		paths []string
	}
	var groups []*fileGroup
	listed := map[string]bool{}
	addFile := func(slug string, ct catalog.ContentType, rel string) {
		prov, ok := providerBySlug(slug)
		if !ok || !typeSet[ct] || listed[string(ct)+"/"+rel] {
			return
		}
		listed[string(ct)+"/"+rel] = true
		var g *fileGroup
		for _, existing := range groups {
			if existing.prov.Slug == slug && existing.ct == ct {
				g = existing
			}
		}
		if g == nil {
			g = &fileGroup{prov: prov, ct: ct}
			groups = append(groups, g)
		}
		g.paths = append(g.paths, filepath.Join(dir, rel))
	}
	for _, pc := range native.Providers {
		for _, ct := range []catalog.ContentType{catalog.Hooks, catalog.MCP} {
			for _, ni := range pc.Items[string(ct)] {
				addFile(pc.ProviderSlug, ct, filepath.ToSlash(ni.Path))
			}
		}
	}
	for _, d := range confirm {
		if rel := settingsFileOf(d); rel != "" {
			addFile(d.Provider, d.Type, rel)
		}
	}

	covered = map[string]bool{}
	seen := map[string]bool{}
	for _, g := range groups {
		settings, errs, err := add.DiscoverSettingsFiles(g.prov, dir, g.paths, contentRoot, g.ct)
		if err != nil {
			errs = []error{err}
		}
		// A file holding both hooks and servers fails both reads alike.
		for _, err := range errs {
			if !seen[err.Error()] {
				seen[err.Error()] = true
				unread = append(unread, err)
			}
		}
		for _, s := range settings {
			covered[string(g.ct)+"/"+relPathOrEmpty(dir, s.Path)] = true
		}
		items = append(items, settingsItemsToDiscovery(settings, g.prov.Slug, func(path string) string {
			return relPathOrEmpty(dir, path)
		})...)
	}
	return items, covered, unread
}

// settingsFileOf returns the settings file, relative to the scan root, that
// an analyzer hook or MCP item was read from, or "" for a hook script.
func settingsFileOf(d *analyzer.DetectedItem) string {
	if d.Type != catalog.Hooks && d.Type != catalog.MCP || d.InternalLabel == "hook-script" {
		return ""
	}
	if d.ConfigSource != "" {
		return filepath.ToSlash(d.ConfigSource)
	}
	return filepath.ToSlash(d.Path)
}

// providerBySlug returns the provider slug names.
func providerBySlug(slug string) (provider.Provider, bool) {
	for _, p := range provider.AllProviders {
		if p.Slug == slug {
			return p, true
		}
	}
	return provider.Provider{}, false
}

// settingsItemsToDiscovery turns settings entries read for provSlug into
// discovery items. relPath renders a settings file's path for the list.
func settingsItemsToDiscovery(settings []add.SettingsItem, provSlug string, relPath func(string) string) []addDiscoveryItem {
	items := make([]addDiscoveryItem, 0, len(settings))
	for i := range settings {
		s := &settings[i]
		item := addDiscoveryItem{
			name:       s.Name,
			itemType:   s.Type,
			path:       s.Path,
			status:     s.Status,
			scope:      s.Scope,
			provider:   provSlug,
			underlying: &s.DiscoveryItem,
			settings:   s,
		}
		if rel := relPath(s.Path); rel != "" {
			item.relativePath = rel + " [" + s.Name + "]"
		}
		if s.Hook != nil {
			item.risks = hookRisks(*s.Hook)
			item.hookData = s.Hook
			item.hookSourceDir = filepath.Dir(s.Path)
		} else {
			item.risks = mcpRisks(gjson.ParseBytes(s.Server))
		}
		items = append(items, item)
	}
	return items
}

// hookRisks reports what a hook does when it runs: a command or a call to
// a URL, whichever its first handler that has one does.
func hookRisks(hook converter.HookData) []catalog.RiskIndicator {
	for _, entry := range hook.Hooks {
		if entry.Command != "" {
			return []catalog.RiskIndicator{{
				Label:       "Runs commands",
				Description: "Hook executes: " + truncate(entry.Command, 60),
				Level:       catalog.RiskHigh,
			}}
		}
		if entry.URL != "" {
			return []catalog.RiskIndicator{{
				Label:       "Network access",
				Description: "Hook calls: " + truncate(entry.URL, 60),
				Level:       catalog.RiskMedium,
			}}
		}
	}
	return nil
}

// mcpRisks reports what an MCP server entry does: launch a process, reach
// a remote endpoint, or receive environment variables.
func mcpRisks(server gjson.Result) []catalog.RiskIndicator {
	var risks []catalog.RiskIndicator
	if cmd := server.Get("command").String(); cmd != "" {
		risks = append(risks, catalog.RiskIndicator{
			Label:       "Runs process",
			Description: "Launches: " + truncate(cmd, 60),
			Level:       catalog.RiskHigh,
		})
	}
	transport := server.Get("type").String()
	if server.Get("url").String() != "" || transport == "sse" || transport == "http" {
		risks = append(risks, catalog.RiskIndicator{
			Label:       "Network access",
			Description: "Connects to remote endpoint",
			Level:       catalog.RiskMedium,
		})
	}
	if len(server.Get("env").Map()) > 0 {
		risks = append(risks, catalog.RiskIndicator{
			Label:       "Environment variables",
			Description: "Server receives environment variables",
			Level:       catalog.RiskMedium,
		})
	}
	return risks
}

// addSettingsEntry adds a hook or MCP server read from a provider's
// settings file, as `syllago add` does. A pinned library item is left as it
// is and reported as pinned. The item lands where discovery placed it, the
// place the review step showed.
func addSettingsEntry(item addDiscoveryItem, contentRoot, projectRoot, srcReg, srcVis, provSlug, srcSHA string) addExecResult {
	s := *item.settings
	// The review step's rename lands in metadata only. A display name that
	// is just the entry's name is left out, so a hook placed at a -N
	// directory keeps that directory's name.
	if item.displayName != s.Name {
		s.DisplayName = item.displayName
	}
	s.Description = item.description
	opts := add.AddOptions{
		Force:            item.overwrite,
		Provider:         provSlug,
		SourceRegistry:   srcReg,
		SourceSHA:        srcSHA,
		SourceVisibility: srcVis,
	}
	plan := opts
	plan.DryRun = true
	planned := add.AddPlacedSettings([]add.SettingsItem{s}, plan, projectRoot, contentRoot)[0]

	var r add.SettingsResult
	_, err := lifecycle.New().Overwrite(lifecycle.OverwriteRequest{
		Destinations: []lifecycle.Destination{{Type: item.itemType, Name: planned.LibraryName, Path: planned.Dest}},
		Write: func([]lifecycle.Destination) ([]lifecycle.Written, error) {
			// The pin check covered planned.Dest alone, so an item the
			// Library moved since the plan is not written elsewhere.
			if again := add.AddPlacedSettings([]add.SettingsItem{s}, plan, projectRoot, contentRoot)[0]; again.Dest != planned.Dest {
				return nil, fmt.Errorf("the Library changed while %s was being added; add it again", item.name)
			}
			r = add.AddPlacedSettings([]add.SettingsItem{s}, opts, projectRoot, contentRoot)[0]
			return []lifecycle.Written{{Path: r.Dest, SourceSHA: srcSHA}}, nil
		},
	})
	var decision *lifecycle.DecisionRequired
	if errors.As(err, &decision) {
		return addExecResult{name: item.name, status: "pinned"}
	}
	if err != nil {
		return addExecResult{name: item.name, status: "error", err: err}
	}
	switch r.Status {
	case add.AddStatusAdded:
		return addExecResult{name: item.name, status: "added"}
	case add.AddStatusUpdated:
		return addExecResult{name: item.name, status: "updated"}
	case add.AddStatusUpToDate, add.AddStatusSkipped:
		return addExecResult{name: item.name, status: "skipped"}
	default:
		return addExecResult{name: item.name, status: "error", err: r.Error}
	}
}
