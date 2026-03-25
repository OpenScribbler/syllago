package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

var addCmd = &cobra.Command{
	Use:   "add [<type>[/<name>]] --from <provider>",
	Short: "Add content to your library from a provider",
	Long: `Discovers content from a provider and adds it to your library (~/.syllago/content/).

Without a positional argument, shows what content is available (discovery mode).
Provide a type or type/name to add content.

Syllago handles format conversion automatically. Once added, content can be
installed to any supported provider with "syllago install --to <provider>".

After adding, use "syllago install" to activate content in a provider.

Hooks-specific flags (--exclude, --scope) are only meaningful when adding hooks.`,
	Example: `  # Discover available content (read-only)
  syllago add --from claude-code

  # Add all rules from a provider
  syllago add rules --from claude-code

  # Add a specific rule by name
  syllago add rules/security --from claude-code

  # Add everything
  syllago add --all --from claude-code

  # Preview what would be written
  syllago add --from claude-code --dry-run`,
	RunE: runAdd,
}

func init() {
	addCmd.Flags().String("from", "", "Provider to add from (required)")
	addCmd.MarkFlagRequired("from")
	addCmd.Flags().Bool("all", false, "Add all discovered content (cannot combine with positional target)")
	addCmd.Flags().Bool("dry-run", false, "Show what would be written without actually writing")
	// Hooks-specific flags — hidden from default help but still functional.
	addCmd.Flags().StringArray("exclude", nil, "Skip hooks by auto-derived name (hooks only)")
	addCmd.Flags().String("scope", "all", "Settings scope to read from: global, project, or all (hooks/mcp only)")
	if err := addCmd.Flags().MarkHidden("exclude"); err == nil {
		_ = addCmd.Flags().MarkHidden("scope")
	}
	addCmd.Flags().BoolP("force", "f", false, "Overwrite existing item without prompting")
	addCmd.Flags().String("base-dir", "", "Override base directory for content discovery")
	addCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
	addCmd.Flags().String("name", "", "Display name for hooks/MCP (stored in .syllago.yaml metadata)")
	// Registry taint propagation — used by internal callers (TUI import, registry add).
	addCmd.Flags().String("source-registry", "", "Registry name for taint tracking")
	addCmd.Flags().String("source-visibility", "", "Source registry visibility (public, private, unknown)")
	_ = addCmd.Flags().MarkHidden("source-registry")
	_ = addCmd.Flags().MarkHidden("source-visibility")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	fromSlug, _ := cmd.Flags().GetString("from")
	if fromSlug == "" {
		return output.NewStructuredError(output.ErrInputMissing, "missing --from flag", "Usage: syllago add [type] --from <provider>")
	}

	// Handle --from shared: add content from the project's shared content directory.
	if fromSlug == "shared" {
		addAll, _ := cmd.Flags().GetBool("all")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")
		return runAddFromShared(root, args, addAll, dryRun, force)
	}

	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		slugs := providerSlugs()
		slugs = append(slugs, "shared")
		return output.NewStructuredError(
			output.ErrProviderNotFound,
			"unknown provider: "+fromSlug,
			"Available: "+strings.Join(slugs, ", "),
		)
	}

	addAll, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	baseDir, _ := cmd.Flags().GetString("base-dir")

	// --all and a positional target are mutually exclusive.
	if addAll && len(args) > 0 {
		return output.NewStructuredError(output.ErrInputConflict, "cannot specify both a target and --all", "Use either a positional argument or --all, not both")
	}

	// Build resolver from merged config + CLI flag.
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigInvalid, "loading global config", "Check ~/.syllago/config.json syntax", err.Error())
	}
	projectCfg, err := config.Load(root)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigNotFound, "loading project config", "Run 'syllago init' to create project config", err.Error())
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigPath, "expanding paths", "Check path overrides in config", err.Error())
	}

	// Warn if the source provider is not detected on disk.
	if !output.JSON && !output.Quiet {
		detected := provider.DetectProvidersWithResolver(resolver)
		for _, dp := range detected {
			if dp.Slug == prov.Slug && !dp.Detected {
				fmt.Fprintf(output.ErrWriter, "Warning: %s not detected at default locations.\n", prov.Name)
				fmt.Fprintf(output.ErrWriter, "If installed at a custom path, configure it:\n")
				fmt.Fprintf(output.ErrWriter, "  syllago config paths --provider %s --path /your/path\n", prov.Slug)
				break
			}
		}
	}

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}

	// No positional arg and no --all → discovery mode.
	if len(args) == 0 && !addAll {
		return runAddDiscovery(root, prov.Slug, resolver, globalDir)
	}

	// With positional arg, parse "type" or "type/name".
	var typeStr, nameFilter string
	if len(args) > 0 {
		parts := strings.SplitN(args[0], "/", 2)
		typeStr = parts[0]
		if len(parts) == 2 {
			nameFilter = parts[1]
		}
	}

	// Hooks and MCP have separate paths because they live in JSON config files.
	displayName, _ := cmd.Flags().GetString("name")
	if typeStr == string(catalog.Hooks) {
		exclude, _ := cmd.Flags().GetStringArray("exclude")
		scope, _ := cmd.Flags().GetString("scope")
		srcReg, _ := cmd.Flags().GetString("source-registry")
		srcVis, _ := cmd.Flags().GetString("source-visibility")
		return runAddHooks(root, fromSlug, dryRun, exclude, force, scope, resolver, srcReg, srcVis, displayName)
	}
	if typeStr == string(catalog.MCP) {
		exclude, _ := cmd.Flags().GetStringArray("exclude")
		scope, _ := cmd.Flags().GetString("scope")
		srcReg, _ := cmd.Flags().GetString("source-registry")
		srcVis, _ := cmd.Flags().GetString("source-visibility")
		return runAddMcp(root, fromSlug, dryRun, exclude, force, scope, resolver, srcReg, srcVis, displayName)
	}

	// For --all, also add hooks and MCP alongside file-based content.
	if addAll {
		scope, _ := cmd.Flags().GetString("scope")
		srcReg, _ := cmd.Flags().GetString("source-registry")
		srcVis, _ := cmd.Flags().GetString("source-visibility")
		if err := runAddHooks(root, fromSlug, dryRun, nil, force, scope, resolver, srcReg, srcVis, ""); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to add hooks: %v\n", err)
		}
		if err := runAddMcp(root, fromSlug, dryRun, nil, force, scope, resolver, srcReg, srcVis, ""); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to add MCP configs: %v\n", err)
		}
	}

	// File-based content: discover and add.
	items, err := add.DiscoverFromProvider(*prov, root, resolver, globalDir)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "discovering content", "Check provider installation and permissions", err.Error())
	}

	// Filter by type when a positional arg is provided (and it's not --all).
	if typeStr != "" {
		ct := catalog.ContentType(typeStr)
		// Validate the type.
		valid := false
		for _, known := range catalog.AllContentTypes() {
			if ct == known {
				valid = true
				break
			}
		}
		if !valid {
			var typeNames []string
			for _, t := range catalog.AllContentTypes() {
				typeNames = append(typeNames, string(t))
			}
			return output.NewStructuredError(output.ErrItemTypeUnknown, fmt.Sprintf("unknown content type %q", typeStr), "Available: "+strings.Join(typeNames, ", "))
		}

		var filtered []add.DiscoveryItem
		for _, item := range items {
			if item.Type == ct {
				filtered = append(filtered, item)
			}
		}
		items = filtered

		// If a specific name was requested, further filter.
		if nameFilter != "" {
			var nameFiltered []add.DiscoveryItem
			for _, item := range items {
				if item.Name == nameFilter {
					nameFiltered = append(nameFiltered, item)
				}
			}
			if len(nameFiltered) == 0 {
				// Build list of available names for the error message.
				var available []string
				for _, item := range items {
					available = append(available, item.Name)
				}
				availStr := strings.Join(available, ", ")
				if availStr == "" {
					availStr = "(none found)"
				}
				return output.NewStructuredError(output.ErrItemNotFound, fmt.Sprintf("no %s named %q found in %s", typeStr, nameFilter, prov.Name), "Available "+typeStr+": "+availStr)
			}
			items = nameFiltered
		}
	}

	// Build the canonicalizer adapter for this content type.
	var canon add.Canonicalizer
	if typeStr != "" {
		if conv := converter.For(catalog.ContentType(typeStr)); conv != nil {
			canon = &converterAdapter{conv: conv, provSlug: fromSlug}
		}
	}

	srcRegistry, _ := cmd.Flags().GetString("source-registry")
	srcVisibility, _ := cmd.Flags().GetString("source-visibility")

	results := add.AddItems(items, add.AddOptions{
		Force:            force,
		DryRun:           dryRun,
		Provider:         fromSlug,
		SourceRegistry:   srcRegistry,
		SourceVisibility: srcVisibility,
	}, globalDir, canon, version)

	return printAddResults(results, dryRun, prov.Name)
}

// converterAdapter adapts converter.Converter to the add.Canonicalizer interface.
type converterAdapter struct {
	conv     converter.Converter
	provSlug string
}

func (a *converterAdapter) Canonicalize(raw []byte, sourceProvider string) ([]byte, string, error) {
	result, err := a.conv.Canonicalize(raw, sourceProvider)
	if err != nil {
		return nil, "", err
	}
	if result == nil || result.Content == nil {
		return nil, "", nil
	}
	return result.Content, result.Filename, nil
}

// printAddResults writes per-item output and the summary line.
func printAddResults(results []add.AddResult, dryRun bool, providerName string) error {
	if output.Quiet {
		return nil
	}

	var added, updated, upToDate, skipped, errCount int
	for _, r := range results {
		switch r.Status {
		case add.AddStatusAdded:
			added++
			if dryRun {
				fmt.Fprintf(output.Writer, "  %-22s [dry-run] would add\n", r.Name)
			} else {
				fmt.Fprintf(output.Writer, "  %-22s added\n", r.Name)
			}
		case add.AddStatusUpdated:
			updated++
			if dryRun {
				fmt.Fprintf(output.Writer, "  %-22s [dry-run] would update\n", r.Name)
			} else {
				fmt.Fprintf(output.Writer, "  %-22s updated\n", r.Name)
			}
		case add.AddStatusUpToDate:
			upToDate++
			fmt.Fprintf(output.Writer, "  %-22s up to date\n", r.Name)
		case add.AddStatusSkipped:
			skipped++
			fmt.Fprintf(output.Writer, "  %-22s source changed (use --force to update)\n", r.Name)
		case add.AddStatusError:
			errCount++
			fmt.Fprintf(output.ErrWriter, "  %-22s error: %v\n", r.Name, r.Error)
		}
	}

	fmt.Fprintln(output.Writer)

	// Summary line with type and provider context.
	typeLabel := summaryTypeLabel(results)
	source := ""
	if providerName != "" {
		source = " from " + providerName
	}

	var parts []string
	if dryRun {
		if added > 0 {
			parts = append(parts, fmt.Sprintf("[dry-run] would add %d %s%s", added, typeLabel, source))
		}
		if updated > 0 {
			parts = append(parts, fmt.Sprintf("would update %d", updated))
		}
	} else {
		if added > 0 {
			parts = append(parts, fmt.Sprintf("Added %d %s%s", added, typeLabel, source))
		}
		if updated > 0 {
			parts = append(parts, fmt.Sprintf("updated %d", updated))
		}
	}
	if upToDate > 0 {
		parts = append(parts, fmt.Sprintf("%d already up to date", upToDate))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d has updates (use --force)", skipped))
	}
	if len(parts) > 0 {
		fmt.Fprintln(output.Writer, strings.Join(parts, ". ")+".")
	}

	return nil
}

// summaryTypeLabel returns a human-readable label for the content types in a
// set of add results. If all items share the same type, it returns that type
// name (e.g., "rules"); otherwise it returns "items".
func summaryTypeLabel(results []add.AddResult) string {
	if len(results) == 0 {
		return "items"
	}
	first := results[0].Type
	for _, r := range results[1:] {
		if r.Type != first {
			return "items"
		}
	}
	return strings.ToLower(string(first))
}

// discoveryGroup is the JSON structure for one content type's discovered items.
type discoveryGroup struct {
	Type  string              `json:"type"`
	Count int                 `json:"count"`
	Items []discoveryJSONItem `json:"items"`
}

// discoveryJSONItem is one item in a JSON discovery group.
type discoveryJSONItem struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Scope  string `json:"scope,omitempty"`
}

// discoveryJSON is the top-level JSON output for discovery mode.
type discoveryJSON struct {
	Provider string           `json:"provider"`
	Groups   []discoveryGroup `json:"groups"`
}

// runAddDiscovery handles "syllago add --from <provider>" with no positional arg.
// It is read-only: it discovers and annotates but never writes.
func runAddDiscovery(root, fromSlug string, resolver *config.PathResolver, globalDir string) error {
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		return output.NewStructuredError(output.ErrProviderNotFound, "unknown provider: "+fromSlug, "Run 'syllago info providers' to see available providers")
	}

	// Discover file-based content.
	items, err := add.DiscoverFromProvider(*prov, root, resolver, globalDir)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "discovering content", "Check provider installation and permissions", err.Error())
	}

	// Discover hooks separately (they live in settings.json, not as files).
	hookItems := discoverHooksForDisplay(root, fromSlug, resolver, globalDir)
	items = append(items, hookItems...)

	// Discover MCP configs separately (they live in JSON config files, not as files).
	mcpItems := discoverMcpForDisplay(root, fromSlug, resolver, globalDir)
	items = append(items, mcpItems...)

	if output.JSON {
		return printDiscoveryJSON(fromSlug, items)
	}

	return printDiscoveryText(fromSlug, prov.Name, items)
}

// discoverHooksForDisplay reads settings.json locations for the provider and
// returns DiscoveryItems for each hook, annotated with library status.
func discoverHooksForDisplay(root, fromSlug string, resolver *config.PathResolver, globalDir string) []add.DiscoveryItem {
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		return nil
	}

	baseDir := ""
	if resolver != nil {
		baseDir = resolver.BaseDir(fromSlug)
	}
	locations, err := installer.FindSettingsLocationsWithBase(*prov, root, baseDir)
	if err != nil {
		return nil
	}

	// Pre-build index for existence check.
	idx, err := add.BuildLibraryIndex(globalDir)
	if err != nil {
		return nil
	}

	var result []add.DiscoveryItem
	for _, loc := range locations {
		data, err := os.ReadFile(loc.Path)
		if err != nil {
			continue
		}
		hooks, err := converter.SplitSettingsHooks(data, fromSlug)
		if err != nil {
			continue
		}
		for _, hook := range hooks {
			name := converter.DeriveHookName(hook)

			key := string(catalog.Hooks) + "/" + fromSlug + "/" + name
			_, inLib := idx[key]
			status := add.StatusNew
			if inLib {
				status = add.StatusInLibrary
			}
			result = append(result, add.DiscoveryItem{
				Name:   name,
				Type:   catalog.Hooks,
				Path:   loc.Path,
				Scope:  loc.Scope.String(),
				Status: status,
			})
		}
	}
	return result
}

// discoverMcpForDisplay reads MCP config locations for the provider and returns
// DiscoveryItems for each server, annotated with library status and scope.
func discoverMcpForDisplay(root, fromSlug string, resolver *config.PathResolver, globalDir string) []add.DiscoveryItem {
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		return nil
	}

	baseDir := ""
	if resolver != nil {
		baseDir = resolver.BaseDir(fromSlug)
	}
	locations := installer.FindMCPLocations(*prov, root, baseDir)

	idx, err := add.BuildLibraryIndex(globalDir)
	if err != nil {
		return nil
	}

	var result []add.DiscoveryItem
	for _, loc := range locations {
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
		servers.ForEach(func(key, _ gjson.Result) bool {
			name := key.String()
			libKey := string(catalog.MCP) + "/" + fromSlug + "/" + name
			_, inLib := idx[libKey]
			status := add.StatusNew
			if inLib {
				status = add.StatusInLibrary
			}
			result = append(result, add.DiscoveryItem{
				Name:   name,
				Type:   catalog.MCP,
				Path:   loc.Path,
				Scope:  loc.Scope.String(),
				Status: status,
			})
			return true
		})
	}
	return result
}

// runAddMcp handles "syllago add mcp --from <provider>". It reads MCP config
// locations, extracts individual server entries, and writes each to the library.
func runAddMcp(root, fromSlug string, previewOnly bool, exclude []string, force bool, scope string, resolver *config.PathResolver, srcRegistry, srcVisibility, displayName string) error {
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		return output.NewStructuredError(output.ErrProviderNotFound, "unknown provider: "+fromSlug, "Run 'syllago info providers' to see available providers")
	}

	baseDir := ""
	if resolver != nil {
		baseDir = resolver.BaseDir(prov.Slug)
	}
	locations := installer.FindMCPLocations(*prov, root, baseDir)

	// Filter by --scope.
	var targets []installer.MCPLocation
	for _, loc := range locations {
		if scope == "all" || loc.Scope.String() == scope {
			targets = append(targets, loc)
		}
	}

	if len(targets) == 0 {
		fmt.Fprintf(output.Writer, "No MCP configs found for %s (scope: %s).\n", fromSlug, scope)
		return nil
	}

	excludeSet := make(map[string]bool, len(exclude))
	for _, ex := range exclude {
		excludeSet[ex] = true
	}

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}

	count := 0
	for _, loc := range targets {
		n, err := addMcpFromLocation(fromSlug, loc, root, previewOnly, excludeSet, force, globalDir, srcRegistry, srcVisibility, displayName)
		if err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to add MCP configs from %s: %v\n", loc.Path, err)
		}
		count += n
	}
	if !previewOnly {
		provLabel := fromSlug
		if prov != nil {
			provLabel = prov.Name
		}
		fmt.Fprintf(output.Writer, "\nAdded %d MCP servers from %s.\n", count, provLabel)
	}
	return nil
}

// addMcpFromLocation reads a single config file, extracts MCP server entries,
// and either previews or writes them to the library.
func addMcpFromLocation(fromSlug string, loc installer.MCPLocation, projectRoot string, previewOnly bool, excludeSet map[string]bool, force bool, globalDir, srcRegistry, srcVisibility, displayName string) (int, error) {
	prov := findProviderBySlug(fromSlug)

	data, err := os.ReadFile(loc.Path)
	if err != nil {
		return 0, output.NewStructuredErrorDetail(output.ErrSystemIO, "reading "+loc.Path, "Check file permissions", err.Error())
	}
	if prov != nil && prov.Slug == "opencode" {
		data = converter.StripJSONCComments(data)
	}

	servers := gjson.GetBytes(data, loc.JSONKey)
	if !servers.Exists() || servers.Type != gjson.JSON {
		return 0, nil
	}

	// Collect server entries.
	type serverEntry struct {
		name string
		raw  string
	}
	var entries []serverEntry
	servers.ForEach(func(key, value gjson.Result) bool {
		name := key.String()
		if !excludeSet[name] {
			entries = append(entries, serverEntry{name: name, raw: value.Raw})
		}
		return true
	})

	if previewOnly {
		fmt.Fprintf(output.Writer, "MCP servers in %s (%s):\n", loc.Path, loc.Scope)
		for _, e := range entries {
			fmt.Fprintf(output.Writer, "  %s\n", e.name)
		}
		fmt.Fprintf(output.Writer, "\n%d MCP servers would be added.\n", len(entries))
		return 0, nil
	}

	scope := loc.Scope.String()
	projectName := ""
	if scope == "project" && projectRoot != "" {
		projectName = filepath.Base(projectRoot)
	}

	count := 0
	for _, e := range entries {
		itemDir := filepath.Join(globalDir, string(catalog.MCP), fromSlug, e.name)

		if !force {
			if info, err := os.Stat(itemDir); err == nil && info.IsDir() {
				existingMeta, _ := metadata.Load(itemDir)
				if existingMeta != nil && existingMeta.SourceScope != scope {
					itemDir = uniqueItemDir(itemDir)
				} else {
					fmt.Fprintf(output.Writer, "  SKIP %s (already exists, use --force to overwrite)\n", e.name)
					continue
				}
			}
		}

		if err := os.MkdirAll(itemDir, 0755); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to create %s: %v\n", itemDir, err)
			continue
		}

		// Write config.json in the nested format expected by the installer.
		configJSON := fmt.Sprintf("{\n  %q: {\n    %q: %s\n  }\n}", loc.JSONKey, e.name, e.raw)
		if err := os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(configJSON), 0644); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to write config.json for %s: %v\n", e.name, err)
			continue
		}

		now := time.Now().UTC()
		mcpMetaName := e.name
		if displayName != "" {
			mcpMetaName = displayName
		}
		meta := &metadata.Meta{
			ID:               metadata.NewID(),
			Name:             mcpMetaName,
			Type:             string(catalog.MCP),
			AddedAt:          &now,
			SourceProvider:   fromSlug,
			SourceFormat:     "json",
			SourceType:       "provider",
			SourceRegistry:   srcRegistry,
			SourceVisibility: srcVisibility,
			SourceScope:      scope,
			SourceProject:    projectName,
		}
		if srcRegistry != "" {
			meta.SourceType = "registry"
		}
		if err := metadata.Save(itemDir, meta); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to write metadata for %s: %v\n", e.name, err)
			continue
		}

		fmt.Fprintf(output.Writer, "  %-22s added (%s)\n", e.name, scope)
		count++
	}
	return count, nil
}

// printDiscoveryJSON outputs structured JSON for discovery mode.
func printDiscoveryJSON(provSlug string, items []add.DiscoveryItem) error {
	// Group items by type.
	groupMap := make(map[catalog.ContentType][]add.DiscoveryItem)
	for _, item := range items {
		groupMap[item.Type] = append(groupMap[item.Type], item)
	}

	var groups []discoveryGroup
	for _, ct := range catalog.AllContentTypes() {
		typeItems, ok := groupMap[ct]
		if !ok {
			continue
		}
		var jsonItems []discoveryJSONItem
		for _, item := range typeItems {
			jsonItems = append(jsonItems, discoveryJSONItem{
				Name:   item.Name,
				Path:   item.Path,
				Status: statusJSONLabel(item.Status),
				Scope:  item.Scope,
			})
		}
		groups = append(groups, discoveryGroup{
			Type:  string(ct),
			Count: len(jsonItems),
			Items: jsonItems,
		})
	}

	result := discoveryJSON{
		Provider: provSlug,
		Groups:   groups,
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "marshaling discovery JSON", "", err.Error())
	}
	fmt.Fprintln(output.Writer, string(data))
	return nil
}

// statusJSONLabel converts ItemStatus to the JSON label used in discovery output.
func statusJSONLabel(s add.ItemStatus) string {
	switch s {
	case add.StatusInLibrary:
		return "in_library"
	case add.StatusOutdated:
		return "outdated"
	default:
		return "new"
	}
}

// printDiscoveryText outputs human-readable discovery output.
func printDiscoveryText(provSlug, provName string, items []add.DiscoveryItem) error {
	if len(items) == 0 {
		fmt.Fprintf(output.Writer, "\nNo content found for %s.\n", provName)
		return nil
	}

	// Group items by type in display order.
	groupMap := make(map[catalog.ContentType][]add.DiscoveryItem)
	for _, item := range items {
		groupMap[item.Type] = append(groupMap[item.Type], item)
	}

	fmt.Fprintf(output.Writer, "\nDiscovered content from %s:\n", provName)
	for _, ct := range catalog.AllContentTypes() {
		typeItems, ok := groupMap[ct]
		if !ok {
			continue
		}
		fmt.Fprintf(output.Writer, "  %s (%d):\n", ct.Label(), len(typeItems))
		for _, item := range typeItems {
			if item.Scope != "" {
				fmt.Fprintf(output.Writer, "    %-20s (%s, %s)\n", item.Name, item.Status.String(), item.Scope)
			} else {
				fmt.Fprintf(output.Writer, "    %-20s (%s)\n", item.Name, item.Status.String())
			}
		}
	}

	// Contextual footer: "Add by type" only lists types with new/outdated items.
	var actionableTypes []catalog.ContentType
	for _, ct := range catalog.AllContentTypes() {
		typeItems, ok := groupMap[ct]
		if !ok {
			continue
		}
		for _, item := range typeItems {
			if item.Status == add.StatusNew || item.Status == add.StatusOutdated {
				actionableTypes = append(actionableTypes, ct)
				break
			}
		}
	}

	// Pick an example item (prefer "new" status).
	var exampleItem *add.DiscoveryItem
	for i := range items {
		if items[i].Status == add.StatusNew {
			exampleItem = &items[i]
			break
		}
	}
	if exampleItem == nil && len(items) > 0 {
		exampleItem = &items[0]
	}

	fmt.Fprintln(output.Writer)
	if len(actionableTypes) > 0 {
		fmt.Fprintln(output.Writer, "Add by type:")
		for _, ct := range actionableTypes {
			fmt.Fprintf(output.Writer, "  syllago add %s --from %s\n", string(ct), provSlug)
		}
		fmt.Fprintln(output.Writer)
	}

	if exampleItem != nil {
		fmt.Fprintln(output.Writer, "Add a specific item:")
		fmt.Fprintf(output.Writer, "  syllago add %s/%s --from %s\n", string(exampleItem.Type), exampleItem.Name, provSlug)
		fmt.Fprintln(output.Writer)
	}

	fmt.Fprintln(output.Writer, "Add everything:")
	fmt.Fprintf(output.Writer, "  syllago add --all --from %s\n", provSlug)
	fmt.Fprintln(output.Writer)

	fmt.Fprintln(output.Writer, "See also:")
	fmt.Fprintln(output.Writer, "  Convert format:    syllago convert <item> --to <provider>")
	fmt.Fprintln(output.Writer, "  Install content:   syllago install <item> --to <provider>")

	return nil
}

// runAddHooks handles "syllago add hooks --from <provider>". It reads settings.json
// for the given provider, splits it into individual hook groups, filters by
// --exclude, and either prints a preview or writes each hook to library.
func runAddHooks(root, fromSlug string, previewOnly bool, exclude []string, force bool, scope string, resolver *config.PathResolver, srcRegistry, srcVisibility, displayName string) error {
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		return output.NewStructuredError(output.ErrProviderNotFound, "unknown provider: "+fromSlug, "Run 'syllago info providers' to see available providers")
	}

	// Use resolver's effective base dir for settings discovery.
	// This respects the full priority chain: CLI --base-dir > config baseDir > default.
	baseDir := ""
	if resolver != nil {
		baseDir = resolver.BaseDir(prov.Slug)
	}
	locations, err := installer.FindSettingsLocationsWithBase(*prov, root, baseDir)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "finding settings locations", "Check provider config directory exists", err.Error())
	}

	// Filter by --scope.
	var targets []installer.SettingsLocation
	for _, loc := range locations {
		if scope == "all" || loc.Scope.String() == scope {
			targets = append(targets, loc)
		}
	}

	if len(targets) == 0 {
		fmt.Fprintf(output.Writer, "No settings.json found for %s (scope: %s).\n", fromSlug, scope)
		return nil
	}

	excludeSet := make(map[string]bool, len(exclude))
	for _, ex := range exclude {
		excludeSet[ex] = true
	}

	for _, loc := range targets {
		if err := addHooksFromLocation(fromSlug, loc, root, previewOnly, excludeSet, force, srcRegistry, srcVisibility, displayName); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to add hooks from %s: %v\n", loc.Path, err)
		}
	}
	return nil
}

// addHooksFromLocation reads a single settings.json, splits it into hooks,
// and either previews or writes them.
func addHooksFromLocation(fromSlug string, loc installer.SettingsLocation, projectRoot string, previewOnly bool, excludeSet map[string]bool, force bool, srcRegistry, srcVisibility, displayName string) error {
	data, err := os.ReadFile(loc.Path)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "reading "+loc.Path, "Check file permissions", err.Error())
	}

	candidates, err := converter.SplitSettingsHooks(data, fromSlug)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConvertParseFailed, "splitting hooks from "+loc.Path, "Check settings.json format", err.Error())
	}

	// Apply --exclude filter.
	var filtered []converter.HookData
	for _, hook := range candidates {
		name := converter.DeriveHookName(hook)
		if !excludeSet[name] {
			filtered = append(filtered, hook)
		}
	}

	if previewOnly {
		fmt.Fprintf(output.Writer, "Hooks in %s (%s):\n", loc.Path, loc.Scope)
		for _, hook := range filtered {
			name := converter.DeriveHookName(hook)
			matcher := hook.Matcher
			if matcher == "" {
				matcher = "*"
			}
			fmt.Fprintf(output.Writer, "  %s   (%s/%s)\n", name, hook.Event, matcher)
		}
		fmt.Fprintf(output.Writer, "\n%d hooks would be added.\n", len(filtered))
		return nil
	}

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}

	scope := loc.Scope.String()
	projectName := ""
	if scope == "project" && projectRoot != "" {
		projectName = filepath.Base(projectRoot)
	}

	count := 0
	for _, hook := range filtered {
		name := converter.DeriveHookName(hook)
		itemDir := filepath.Join(globalDir, string(catalog.Hooks), fromSlug, name)

		// Handle name collisions: if directory exists and belongs to a different scope,
		// find a unique name by appending -2, -3, etc.
		if !force {
			if info, err := os.Stat(itemDir); err == nil && info.IsDir() {
				existingMeta, _ := metadata.Load(itemDir)
				if existingMeta != nil && existingMeta.SourceScope != scope {
					// Different scope — find unique suffix.
					itemDir = uniqueItemDir(itemDir)
					name = filepath.Base(itemDir)
				} else {
					fmt.Fprintf(output.Writer, "  SKIP %s (already exists, use --force to overwrite)\n", name)
					continue
				}
			}
		}

		if err := os.MkdirAll(itemDir, 0755); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to create %s: %v\n", itemDir, err)
			continue
		}

		hookJSON, err := json.MarshalIndent(hook, "", "  ")
		if err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to marshal hook %s: %v\n", name, err)
			continue
		}
		if err := os.WriteFile(filepath.Join(itemDir, "hook.json"), hookJSON, 0644); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to write hook.json for %s: %v\n", name, err)
			continue
		}

		now := time.Now().UTC()
		metaName := name
		if displayName != "" {
			metaName = displayName
		}
		meta := &metadata.Meta{
			ID:               metadata.NewID(),
			Name:             metaName,
			Type:             string(catalog.Hooks),
			AddedAt:          &now,
			SourceProvider:   fromSlug,
			SourceFormat:     "json",
			SourceType:       "provider",
			SourceRegistry:   srcRegistry,
			SourceVisibility: srcVisibility,
			SourceScope:      scope,
			SourceProject:    projectName,
		}
		if srcRegistry != "" {
			meta.SourceType = "registry"
		}
		if err := metadata.Save(itemDir, meta); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to write metadata for %s: %v\n", name, err)
			continue
		}

		matcher := hook.Matcher
		if matcher == "" {
			matcher = "*"
		}
		fmt.Fprintf(output.Writer, "  %s   (%s/%s, %s)\n", name, hook.Event, matcher, scope)
		count++
	}
	prov := findProviderBySlug(fromSlug)
	provLabel := fromSlug
	if prov != nil {
		provLabel = prov.Name
	}
	fmt.Fprintf(output.Writer, "\nAdded %d hooks from %s.\n", count, provLabel)
	return nil
}

// uniqueItemDir returns a unique directory path by appending -2, -3, etc.
// runAddFromShared copies items from the project's shared content directory to the user's library.
func runAddFromShared(projectRoot string, args []string, addAll, dryRun, force bool) error {
	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}

	// Scan the project root to find shared content.
	cat, err := catalog.Scan(projectRoot, projectRoot)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning project content failed", "Check that the content directories exist", err.Error())
	}

	// Filter to shared items (non-library, non-builtin).
	var items []catalog.ContentItem
	for _, item := range cat.Items {
		if item.Library || item.IsBuiltin() {
			continue
		}
		if len(args) > 0 && item.Name != args[0] {
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		if len(args) > 0 {
			return output.NewStructuredError(output.ErrItemNotFound, fmt.Sprintf("no shared item named %q found", args[0]), "Run 'syllago list --source shared' to see available items")
		}
		fmt.Fprintln(output.ErrWriter, "No shared content found in this project.")
		return nil
	}

	if !addAll && len(args) == 0 {
		// Discovery mode: show what's available.
		fmt.Fprintf(output.Writer, "Shared content in %s:\n", filepath.Base(projectRoot))
		for _, item := range items {
			fmt.Fprintf(output.Writer, "  %s (%s)\n", item.Name, item.Type.Label())
		}
		fmt.Fprintf(output.Writer, "\n%d item(s). Use --all to add everything, or specify a name.\n", len(items))
		return nil
	}

	if dryRun {
		fmt.Fprintf(output.Writer, "[dry-run] would add %d item(s) from shared content:\n", len(items))
		for _, item := range items {
			fmt.Fprintf(output.Writer, "  %s (%s)\n", item.Name, item.Type.Label())
		}
		return nil
	}

	count := 0
	for _, item := range items {
		destDir := filepath.Join(globalDir, string(item.Type), item.Name)

		if !force {
			if _, err := os.Stat(destDir); err == nil {
				fmt.Fprintf(output.Writer, "  SKIP %s (already in library, use --force to overwrite)\n", item.Name)
				continue
			}
		}

		if err := installer.CopyContent(item.Path, destDir); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to copy %s: %v\n", item.Name, err)
			continue
		}

		// Write metadata.
		now := time.Now().UTC()
		meta := &metadata.Meta{
			ID:           metadata.NewID(),
			Name:         item.Name,
			Type:         string(item.Type),
			AddedAt:      &now,
			SourceType:   "shared",
			SourceFormat: "md",
		}
		if err := metadata.Save(destDir, meta); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to write metadata for %s: %v\n", item.Name, err)
		}

		fmt.Fprintf(output.Writer, "  Added %s (%s)\n", item.Name, item.Type.Label())
		count++
	}

	fmt.Fprintf(output.Writer, "\nAdded %d item(s) from shared content.\n", count)
	return nil
}

func uniqueItemDir(base string) string {
	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return base + "-overflow"
}
