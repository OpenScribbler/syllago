package tui

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/tidwall/gjson"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// discoverSettingsFromProvider reads prov's settings files for the hooks or
// MCP servers (ct picks which) and returns one discovery item per entry,
// placed and annotated with library status by add.DiscoverSettings. A
// settings file that could not be read is returned in unread.
func discoverSettingsFromProvider(prov provider.Provider, projectRoot, baseDir, contentRoot string, ct catalog.ContentType) ([]addDiscoveryItem, []error) {
	settings, unread, err := add.DiscoverSettings(prov, projectRoot, baseDir, contentRoot, ct)
	if err != nil {
		return nil, []error{err}
	}
	homeDir, _ := os.UserHomeDir()
	items := make([]addDiscoveryItem, 0, len(settings))
	for i := range settings {
		s := &settings[i]
		item := addDiscoveryItem{
			name:       s.Name,
			itemType:   s.Type,
			path:       s.Path,
			status:     s.Status,
			scope:      s.Scope,
			underlying: &s.DiscoveryItem,
			settings:   s,
		}
		if rel := providerRelPath(s.Path, "", homeDir); rel != "" {
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
	return items, unread
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
// is and reported as pinned.
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
	planned := add.AddFromSettings([]add.SettingsItem{s}, plan, projectRoot, contentRoot)[0]

	var r add.SettingsResult
	_, err := lifecycle.New().Overwrite(lifecycle.OverwriteRequest{
		Destinations: []lifecycle.Destination{{Type: item.itemType, Name: planned.LibraryName, Path: planned.Dest}},
		Write: func([]lifecycle.Destination) ([]lifecycle.Written, error) {
			r = add.AddFromSettings([]add.SettingsItem{s}, opts, projectRoot, contentRoot)[0]
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
	case add.AddStatusUpToDate:
		return addExecResult{name: item.name, status: "skipped"}
	default:
		return addExecResult{name: item.name, status: "error", err: r.Error}
	}
}
