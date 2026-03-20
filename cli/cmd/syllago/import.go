package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/parse"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Bring content into syllago from a provider, path, or git URL",
	Long: `Discovers content from a provider and imports it into your library.

Syllago handles format conversion automatically. Once imported, content can be
installed to any supported provider with "syllago install --to <provider>".

After import, use "syllago install" to activate content in other providers,
or browse in the TUI with "syllago".`,
	Example: `  # Import all content from Claude Code
  syllago import --from claude-code

  # Import only skills
  syllago import --from claude-code --type skills

  # Import a specific rule by name
  syllago import --from cursor --name my-rule

  # Preview what would be imported
  syllago import --from claude-code --preview

  # Show what would be written without writing
  syllago import --from claude-code --dry-run`,
	RunE: runImport,
}

func init() {
	importCmd.Flags().String("from", "", "Provider to import from (required)")
	importCmd.MarkFlagRequired("from")
	importCmd.Flags().String("type", "", "Limit to a single content type (e.g., rules, hooks, mcp)")
	importCmd.Flags().String("name", "", "Filter to items whose path contains this substring (case-insensitive)")
	importCmd.Flags().Bool("preview", false, "Show discovery report without parsing")
	importCmd.Flags().Bool("dry-run", false, "Show what would be written without actually writing")
	// Hooks-specific flags.
	importCmd.Flags().StringArray("exclude", nil, "Skip hooks by auto-derived name (hooks only)")
	importCmd.Flags().Bool("force", false, "Overwrite existing items with the same name (hooks only)")
	importCmd.Flags().String("scope", "global", "Settings scope to read from: global, project, or all (hooks only)")
	importCmd.Flags().String("base-dir", "", "Override base directory for content discovery")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	fromSlug, _ := cmd.Flags().GetString("from")
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		slugs := providerSlugs()
		se := output.NewStructuredError(
			output.ErrProviderNotFound,
			"unknown provider: "+fromSlug,
			"Available: "+strings.Join(slugs, ", "),
		)
		output.PrintStructuredError(se)
		return output.SilentError(se)
	}

	typeFilter, _ := cmd.Flags().GetString("type")
	nameFilter, _ := cmd.Flags().GetString("name")
	preview, _ := cmd.Flags().GetBool("preview")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	baseDir, _ := cmd.Flags().GetString("base-dir")

	// Build resolver from merged config + CLI flag.
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}
	projectCfg, err := config.Load(root)
	if err != nil {
		return fmt.Errorf("loading project config: %w", err)
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return fmt.Errorf("expanding paths: %w", err)
	}

	// Hooks are stored in settings.json and must be split into individual items.
	// The normal file-discovery flow treats the entire settings.json as one item,
	// which is wrong for hooks. Handle this type separately.
	if typeFilter == string(catalog.Hooks) {
		exclude, _ := cmd.Flags().GetStringArray("exclude")
		force, _ := cmd.Flags().GetBool("force")
		scope, _ := cmd.Flags().GetString("scope")
		return runImportHooks(root, fromSlug, preview || dryRun, exclude, force, scope, resolver)
	}

	report := parse.DiscoverWithResolver(*prov, root, resolver)

	if typeFilter != "" {
		ct := catalog.ContentType(typeFilter)
		var filtered []parse.DiscoveredFile
		for _, f := range report.Files {
			if f.ContentType == ct {
				filtered = append(filtered, f)
			}
		}
		report.Files = filtered
		newCounts := map[catalog.ContentType]int{ct: report.Counts[ct]}
		report.Counts = newCounts
	}

	if nameFilter != "" {
		nameFilter = strings.ToLower(nameFilter)
		var filtered []parse.DiscoveredFile
		for _, f := range report.Files {
			if strings.Contains(strings.ToLower(f.Path), nameFilter) {
				filtered = append(filtered, f)
			}
		}
		report.Files = filtered
		// Rebuild counts from filtered files.
		newCounts := make(map[catalog.ContentType]int)
		for _, f := range filtered {
			newCounts[f.ContentType]++
		}
		report.Counts = newCounts
	}

	if preview || output.JSON {
		if output.JSON {
			output.Print(report)
		} else {
			printDiscoveryReport(report)
		}
		return nil
	}

	// Discover + parse, then write canonicalized content to local/.
	parser := parse.ParserForProvider(prov.Slug)
	result := &parse.ImportResult{
		Provider: prov.Slug,
		Report:   report,
	}
	for _, file := range report.Files {
		sections, err := parser.ParseFile(file)
		if err != nil {
			report.Unclassified = append(report.Unclassified, file.Path)
			continue
		}
		result.Sections = append(result.Sections, sections...)
	}

	// Write each discovered file to local/<type>/[<provider>/]<name>/
	var written int
	for _, file := range report.Files {
		dest, err := writeImportedContent(root, file, fromSlug, dryRun)
		if err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to import %s: %v\n", file.Path, err)
			continue
		}
		written++
		if dryRun {
			fmt.Fprintf(output.Writer, "[dry-run] Would write %s -> %s\n", file.Path, dest)
		} else {
			fmt.Fprintf(output.Writer, "Imported %s -> %s\n", file.Path, dest)
		}
	}

	if !dryRun {
		printDiscoveryReport(report)
		fmt.Fprintf(output.Writer, "\nImported %d file(s) to library.\n", written)
	} else {
		printDiscoveryReport(report)
		fmt.Fprintf(output.Writer, "\n[dry-run] Would import %d file(s) to library.\n", written)
	}

	return nil
}

// writeImportedContent reads a discovered file, optionally canonicalizes it,
// and writes it to local/<type>/[<provider>/]<name>/. Returns the destination
// directory path. If dryRun is true, returns the path without writing.
func writeImportedContent(projectRoot string, file parse.DiscoveredFile, sourceProvider string, dryRun bool) (string, error) {
	ct := file.ContentType
	name := itemNameFromPath(file.Path)

	// Build destination: local/<type>/<name>/ for universal types,
	// local/<type>/<provider>/<name>/ for provider-specific types.
	var destDir string
	if ct.IsUniversal() {
		destDir = filepath.Join(projectRoot, "local", string(ct), name)
	} else {
		destDir = filepath.Join(projectRoot, "local", string(ct), sourceProvider, name)
	}

	if dryRun {
		return destDir, nil
	}

	// Read the raw file content.
	raw, err := os.ReadFile(file.Path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", file.Path, err)
	}

	// Canonicalize if a converter exists for this content type.
	content := raw
	ext := filepath.Ext(file.Path)
	if conv := converter.For(ct); conv != nil {
		result, canonErr := conv.Canonicalize(raw, sourceProvider)
		if canonErr == nil && result.Content != nil {
			content = result.Content
			// Use the converter's suggested filename extension if available.
			if result.Filename != "" {
				ext = filepath.Ext(result.Filename)
			}
		}
		// On canonicalize error, fall through and write raw content.
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("creating directory %s: %w", destDir, err)
	}

	// Determine the content filename.
	contentFilename := contentFileForType(ct, name, ext)
	contentPath := filepath.Join(destDir, contentFilename)

	if err := os.WriteFile(contentPath, content, 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", contentPath, err)
	}

	// Write .syllago.yaml with import metadata.
	now := time.Now().UTC()
	sourceExt := strings.TrimPrefix(filepath.Ext(file.Path), ".")
	meta := &metadata.Meta{
		ID:             metadata.NewID(),
		Name:           name,
		Type:           string(ct),
		AddedAt:        &now,
		SourceProvider: sourceProvider,
		SourceFormat:   sourceExt,
	}
	if err := metadata.Save(destDir, meta); err != nil {
		return "", fmt.Errorf("writing metadata: %w", err)
	}

	return destDir, nil
}

// itemNameFromPath derives an item name from a file path.
// Uses the filename without extension (e.g. "security.md" -> "security").
func itemNameFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// contentFileForType returns the canonical content filename for a content type.
func contentFileForType(ct catalog.ContentType, name string, ext string) string {
	if ext == "" {
		ext = ".md"
	}
	switch ct {
	case catalog.Rules:
		return "rule" + ext
	case catalog.Hooks:
		return "hook.json"
	case catalog.Commands:
		return "command" + ext
	case catalog.Skills:
		return "SKILL.md"
	case catalog.Agents:
		return "agent.md"
	case catalog.MCP:
		return "mcp.json"
	default:
		return name + ext
	}
}

func printDiscoveryReport(report parse.DiscoveryReport) {
	fmt.Fprintf(output.Writer, "Import from %s:\n", report.Provider)
	total := 0
	for ct, count := range report.Counts {
		if count > 0 {
			fmt.Fprintf(output.Writer, "  %s: %d file(s)\n", ct.Label(), count)
			total += count
		}
	}
	if total == 0 {
		fmt.Fprintln(output.Writer, "  No content found.")
		printDiscoveryDiagnostics(report)
	}
	if len(report.Unclassified) > 0 {
		fmt.Fprintf(output.Writer, "  %d file(s) couldn't be classified.\n", len(report.Unclassified))
	}
}

// runImportHooks handles "syllago import --type hooks". It reads settings.json
// for the given provider, splits it into individual hook groups, filters by
// --exclude, and either prints a preview or writes each hook to local/.
func runImportHooks(root, fromSlug string, previewOnly bool, exclude []string, force bool, scope string, resolver *config.PathResolver) error {
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		return fmt.Errorf("unknown provider: %s", fromSlug)
	}

	// Use resolver's effective base dir for settings discovery.
	// This respects the full priority chain: CLI --base-dir > config baseDir > default.
	baseDir := ""
	if resolver != nil {
		baseDir = resolver.BaseDir(prov.Slug)
	}
	locations, err := installer.FindSettingsLocationsWithBase(*prov, root, baseDir)
	if err != nil {
		return fmt.Errorf("finding settings locations: %w", err)
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
		if err := importHooksFromLocation(root, fromSlug, loc, previewOnly, excludeSet, force); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to import hooks from %s: %v\n", loc.Path, err)
		}
	}
	return nil
}

// importHooksFromLocation reads a single settings.json, splits it into hooks,
// and either previews or writes them.
func importHooksFromLocation(root, fromSlug string, loc installer.SettingsLocation, previewOnly bool, excludeSet map[string]bool, force bool) error {
	data, err := os.ReadFile(loc.Path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", loc.Path, err)
	}

	candidates, err := converter.SplitSettingsHooks(data, fromSlug)
	if err != nil {
		return fmt.Errorf("splitting hooks from %s: %w", loc.Path, err)
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
		fmt.Fprintf(output.Writer, "\n%d hooks would be imported.\n", len(filtered))
		return nil
	}

	count := 0
	for _, hook := range filtered {
		name := converter.DeriveHookName(hook)
		itemDir := filepath.Join(root, "local", string(catalog.Hooks), fromSlug, name)

		if !force {
			if _, err := os.Stat(itemDir); err == nil {
				fmt.Fprintf(output.Writer, "  SKIP %s (already exists, use --force to overwrite)\n", name)
				continue
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
		meta := &metadata.Meta{
			ID:             metadata.NewID(),
			Name:           name,
			Type:           string(catalog.Hooks),
			AddedAt:        &now,
			SourceProvider: fromSlug,
			SourceFormat:   "json",
		}
		if err := metadata.Save(itemDir, meta); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to write metadata for %s: %v\n", name, err)
			continue
		}

		matcher := hook.Matcher
		if matcher == "" {
			matcher = "*"
		}
		fmt.Fprintf(output.Writer, "  %s   (%s/%s)\n", name, hook.Event, matcher)
		count++
	}
	fmt.Fprintf(output.Writer, "\nImported %d hooks from %s\n", count, fromSlug)
	return nil
}

// printDiscoveryDiagnostics explains why no content was found by showing
// which types aren't supported and which paths were searched but empty.
func printDiscoveryDiagnostics(report parse.DiscoveryReport) {
	// Show unsupported types so the user knows they can't import those.
	for _, ct := range report.Unsupported {
		fmt.Fprintf(output.Writer, "  Note: %s is not supported for %s\n", ct.Label(), report.Provider)
	}
	// Show searched paths for supported types that came back empty.
	for ct, paths := range report.SearchedPaths {
		if report.Counts[ct] == 0 {
			fmt.Fprintf(output.Writer, "  No %s found in %s. Searched: %s\n", ct.Label(), report.Provider, strings.Join(paths, ", "))
		}
	}
}
