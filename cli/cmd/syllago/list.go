package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/ccplugin"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List content items in the library",
	Long: `Show a quick inventory of all content without launching the TUI.

By default, lists all content grouped by type. Use flags to filter.`,
	Example: `  # List all content grouped by type
  syllago list

  # Show only library items
  syllago list --source library

  # Show only skills
  syllago list --type skills

  # JSON output for scripting
  syllago list --json`,
	RunE: runList,
}

// loadCCPlugins is a seam so tests can point plugin discovery at a fixture home.
var loadCCPlugins = func() ([]ccplugin.Plugin, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return ccplugin.Load(home)
}

func init() {
	listCmd.Flags().String("source", "all", "Filter by source: library, shared, registry, builtin, plugin, all")
	listCmd.Flags().String("type", "", "Filter to one content type (e.g., skills, rules)")
	listCmd.Flags().StringSlice("filter", nil, "Filter by item state (repeatable): in-library, not-in-library, project")
	rootCmd.AddCommand(listCmd)
}

// listResult is the JSON-serializable output for syllago list.
type listResult struct {
	Groups  []listGroup       `json:"groups"`
	Plugins []pluginListEntry `json:"plugins,omitempty"`
}

type listGroup struct {
	Type  string     `json:"type"`
	Count int        `json:"count"`
	Items []listItem `json:"items"`
}

type listItem struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Description string `json:"description,omitempty"`
	Trust       string `json:"trust,omitempty"`      // "Verified" / "Revoked" / ""
	TrustTier   string `json:"trust_tier,omitempty"` // full tier for drill-down ("Dual-Attested" etc.)
	Revoked     bool   `json:"revoked,omitempty"`
}

type pluginListEntry struct {
	Name        string   `json:"name"`
	Marketplace string   `json:"marketplace"`
	Version     string   `json:"version,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	Skills      []string `json:"skills,omitempty"`
}

func runList(cmd *cobra.Command, args []string) error {
	root, err := requireContentRepoRoot()
	if err != nil {
		return err
	}

	sourceFilter, _ := cmd.Flags().GetString("source")
	typeFilter, _ := cmd.Flags().GetString("type")
	rawStates, _ := cmd.Flags().GetStringSlice("filter")

	// Strip empty strings left by test cleanup (defer Set("filter", "")).
	var filterStates []string
	for _, s := range rawStates {
		if s != "" {
			filterStates = append(filterStates, s)
		}
	}

	scan, err := loadTrustedScan(root, resolveProjectRoot(root))
	if err != nil {
		return err
	}
	cat := scan.Catalog

	plugins, err := loadCCPlugins()
	if err != nil {
		fmt.Fprintf(output.ErrWriter, "warning: could not read Claude Code plugins: %v\n", err)
		plugins = nil
	}
	enabledPlugins := enabledCCPlugins(plugins)

	var librarySkillNames []string
	for _, item := range cat.ByType(catalog.Skills) {
		if item.Library {
			librarySkillNames = append(librarySkillNames, item.Name)
		}
	}
	for _, c := range ccplugin.SkillCollisions(plugins, librarySkillNames) {
		cat.Warnings = append(cat.Warnings, fmt.Sprintf("plugin skill %q from Claude Code plugin %s shadows a library skill of the same name", c.SkillName, c.PluginID))
	}
	cat.PrintWarnings()

	// Build grouped output across all content types.
	var result listResult
	for _, ct := range catalog.AllContentTypes() {
		if typeFilter != "" && string(ct) != typeFilter {
			continue
		}

		var items []listItem
		for _, item := range cat.ByType(ct) {
			if !filterBySource(item, sourceFilter) {
				continue
			}
			if len(filterStates) > 0 && !filterByState(item, filterStates) {
				continue
			}
			badge := catalog.UserFacingBadge(item.TrustTier, item.Revoked)
			items = append(items, listItem{
				Name:        item.Name,
				Source:      sourceLabel(item),
				Description: item.Description,
				Trust:       badge.Label(),
				TrustTier:   item.TrustTier.String(),
				Revoked:     item.Revoked,
			})
		}

		if len(items) == 0 {
			continue
		}
		result.Groups = append(result.Groups, listGroup{
			Type:  ct.Label(),
			Count: len(items),
			Items: items,
		})
	}

	showPlugins := typeFilter == "" && (sourceFilter == "all" || sourceFilter == "plugin")
	if showPlugins {
		result.Plugins = pluginListEntries(enabledPlugins)
	}

	totalItems := 0
	for _, g := range result.Groups {
		totalItems += g.Count
	}
	telemetry.Enrich("source_filter", sourceFilter)
	telemetry.Enrich("content_type", typeFilter)
	telemetry.Enrich("item_count", totalItems)
	telemetry.Enrich("plugin_count", len(enabledPlugins))
	if len(filterStates) > 0 {
		telemetry.Enrich("filter", strings.Join(filterStates, ","))
	}

	if output.JSON {
		output.Print(result)
		return nil
	}

	printedPlugins := showPlugins && len(enabledPlugins) > 0
	if len(result.Groups) == 0 && !printedPlugins {
		if showPlugins && sourceFilter == "plugin" {
			fmt.Fprintln(output.ErrWriter, "No plugins found.")
			return nil
		}
		fmt.Fprintln(output.ErrWriter, "No items found.")
		return nil
	}

	for i, group := range result.Groups {
		if i > 0 {
			fmt.Fprintln(output.Writer)
		}
		fmt.Fprintf(output.Writer, "%s (%d)\n", group.Type, group.Count)
		for _, item := range group.Items {
			// Trust glyph prefix (2-char column, empty for no-badge items)
			// keeps unaligned Verified/Recalled rows distinguishable at a glance.
			glyph := trustGlyph(item.Trust)
			fmt.Fprintf(output.Writer, "  %-2s %-18s [%-8s] %s\n",
				glyph, item.Name, item.Source, item.Description)
		}
	}

	if printedPlugins {
		if len(result.Groups) > 0 {
			fmt.Fprintln(output.Writer)
		}
		fmt.Fprintf(output.Writer, "Claude Code plugins (%d)\n", len(enabledPlugins))
		for _, plugin := range enabledPlugins {
			line := fmt.Sprintf("  %-28s %s", pluginDisplayID(plugin), pluginDisplayDetail(plugin))
			fmt.Fprintln(output.Writer, strings.TrimRight(line, " "))
		}
	}

	return nil
}

func enabledCCPlugins(plugins []ccplugin.Plugin) []ccplugin.Plugin {
	enabled := make([]ccplugin.Plugin, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin.Enabled {
			enabled = append(enabled, plugin)
		}
	}
	sort.Slice(enabled, func(i, j int) bool {
		if enabled[i].ID != enabled[j].ID {
			return enabled[i].ID < enabled[j].ID
		}
		if enabled[i].Scope != enabled[j].Scope {
			return enabled[i].Scope < enabled[j].Scope
		}
		return enabled[i].InstallPath < enabled[j].InstallPath
	})
	return enabled
}

func pluginListEntries(plugins []ccplugin.Plugin) []pluginListEntry {
	if len(plugins) == 0 {
		return nil
	}
	entries := make([]pluginListEntry, 0, len(plugins))
	for _, plugin := range plugins {
		entries = append(entries, pluginListEntry{
			Name:        plugin.Name,
			Marketplace: plugin.Marketplace,
			Version:     plugin.Version,
			Scope:       plugin.Scope,
			Skills:      pluginSkillNames(plugin),
		})
	}
	return entries
}

func pluginSkillNames(plugin ccplugin.Plugin) []string {
	if len(plugin.Skills) == 0 {
		return nil
	}
	names := make([]string, 0, len(plugin.Skills))
	for _, skill := range plugin.Skills {
		names = append(names, skill.Name)
	}
	return names
}

func pluginDisplayID(plugin ccplugin.Plugin) string {
	if plugin.ID != "" {
		return plugin.ID
	}
	if plugin.Marketplace == "" {
		return plugin.Name
	}
	return plugin.Name + "@" + plugin.Marketplace
}

func pluginDisplayDetail(plugin ccplugin.Plugin) string {
	var parts []string
	// The ledger's version field is opaque and sometimes literally "unknown" —
	// rendering "vunknown" helps no one.
	if plugin.Version != "" && plugin.Version != "unknown" {
		parts = append(parts, "v"+plugin.Version)
	}
	if skills := pluginSkillNames(plugin); len(skills) > 0 {
		parts = append(parts, "skills: "+strings.Join(skills, ", "))
	}
	return strings.Join(parts, "  ")
}

// trustGlyph maps the user-facing trust label to a single-character glyph for
// text list output. Empty string when the item has no badge so the row column
// renders blank rather than a placeholder.
func trustGlyph(label string) string {
	switch label {
	case "Verified":
		return "\u2713"
	case "Revoked":
		return "R"
	}
	return ""
}

// sourceLabel returns a human-readable source tag for display.
func sourceLabel(item catalog.ContentItem) string {
	switch {
	case item.IsBuiltin():
		return "builtin"
	case item.Registry != "":
		return "registry"
	case item.Library:
		return "library"
	default:
		return "shared"
	}
}
