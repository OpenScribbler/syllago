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

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add content to your library from a provider",
	Long: `Discovers content from a provider and adds it to your library (~/.syllago/content/).

Syllago handles format conversion automatically. Once added, content can be
installed to any supported provider with "syllago install --to <provider>".

Examples:
  syllago add --from claude-code                  Add all content from Claude Code
  syllago add --from claude-code --type skills    Add only skills
  syllago add --from cursor --name my-rule        Add a specific rule by name
  syllago add --from claude-code --preview        Preview what would be added (read-only)
  syllago add --from claude-code --dry-run        Show what would be written without writing

After adding, use "syllago install" to activate content in a provider.`,
	RunE: runAdd,
}

func init() {
	addCmd.Flags().String("from", "", "Provider to add from (required)")
	addCmd.MarkFlagRequired("from")
	addCmd.Flags().String("type", "", "Limit to a single content type (e.g., rules, hooks, mcp)")
	addCmd.Flags().String("name", "", "Filter to items whose path contains this substring (case-insensitive)")
	addCmd.Flags().Bool("preview", false, "Show discovery report without parsing")
	addCmd.Flags().Bool("dry-run", false, "Show what would be written without actually writing")
	// Hooks-specific flags.
	addCmd.Flags().StringArray("exclude", nil, "Skip hooks by auto-derived name (hooks only)")
	addCmd.Flags().BoolP("force", "f", false, "Overwrite existing item without prompting")
	addCmd.Flags().String("scope", "global", "Settings scope to read from: global, project, or all (hooks only)")
	addCmd.Flags().String("base-dir", "", "Override base directory for content discovery")
	addCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	fromSlug, _ := cmd.Flags().GetString("from")
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		slugs := providerSlugs()
		output.PrintError(1, "unknown provider: "+fromSlug, "Available: "+strings.Join(slugs, ", "))
		return fmt.Errorf("unknown provider: %s", fromSlug)
	}

	typeFilter, _ := cmd.Flags().GetString("type")
	nameFilter, _ := cmd.Flags().GetString("name")
	preview, _ := cmd.Flags().GetBool("preview")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	baseDir, _ := cmd.Flags().GetString("base-dir")
	force, _ := cmd.Flags().GetBool("force")
	noInput, _ := cmd.Flags().GetBool("no-input")

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
		scope, _ := cmd.Flags().GetString("scope")
		return runAddHooks(root, fromSlug, preview || dryRun, exclude, force, scope, resolver)
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
			printAddDiscoveryReport(report)
		}
		return nil
	}

	// Discover + parse, then write canonicalized content to ~/.syllago/content/.
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

	// Write each discovered file to ~/.syllago/content/<type>/[<provider>/]<name>/
	var written int
	for _, file := range report.Files {
		dest, err := writeAddedContent(file, fromSlug, dryRun, force, noInput)
		if err != nil {
			if err.Error() == "cancelled" {
				if !output.Quiet {
					fmt.Fprintf(output.Writer, "  SKIP %s (cancelled)\n", filepath.Base(file.Path))
				}
				continue
			}
			fmt.Fprintf(output.ErrWriter, "Warning: failed to add %s: %v\n", file.Path, err)
			continue
		}
		written++
		if !output.Quiet {
			if dryRun {
				fmt.Fprintf(output.Writer, "[dry-run] Would write %s -> %s\n", file.Path, dest)
			} else {
				fmt.Fprintf(output.Writer, "Added %s -> %s\n", file.Path, dest)
			}
		}
	}

	if !output.Quiet {
		if !dryRun {
			printAddDiscoveryReport(report)
			fmt.Fprintf(output.Writer, "\nAdded %d item(s) to library.\n", written)
		} else {
			printAddDiscoveryReport(report)
			fmt.Fprintf(output.Writer, "\n[dry-run] Would add %d item(s) to library.\n", written)
		}
	}

	if written > 0 && !dryRun && !output.Quiet {
		fmt.Fprintf(output.Writer, "\n  Next: syllago install <name> --to <provider>\n")
	}

	return nil
}

// writeAddedContent reads a discovered file, optionally canonicalizes it,
// and writes it to ~/.syllago/content/<type>/[<provider>/]<name>/. Returns the
// destination directory path. If dryRun is true, returns the path without writing.
func writeAddedContent(file parse.DiscoveredFile, sourceProvider string, dryRun bool, force bool, noInput bool) (string, error) {
	ct := file.ContentType
	name := itemNameFromPath(file.Path)

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return "", fmt.Errorf("cannot determine home directory")
	}

	// Build destination: <globalDir>/<type>/<name>/ for universal types,
	// <globalDir>/<type>/<provider>/<name>/ for provider-specific types.
	var destDir string
	if ct.IsUniversal() {
		destDir = filepath.Join(globalDir, string(ct), name)
	} else {
		destDir = filepath.Join(globalDir, string(ct), sourceProvider, name)
	}

	if dryRun {
		return destDir, nil
	}

	// Check if item already exists and handle overwrite logic.
	if _, err := os.Stat(destDir); err == nil && !force {
		if noInput || !isInteractive() {
			// Non-interactive: skip (don't overwrite without --force).
			return destDir, fmt.Errorf("cancelled")
		}
		fmt.Fprintf(output.Writer, "Overwrite existing %q? [y/N] ", name)
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			return destDir, fmt.Errorf("cancelled")
		}
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
		convResult, canonErr := conv.Canonicalize(raw, sourceProvider)
		if canonErr == nil && convResult.Content != nil {
			content = convResult.Content
			// Use the converter's suggested filename extension if available.
			if convResult.Filename != "" {
				ext = filepath.Ext(convResult.Filename)
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

	// Preserve original in .source/ if source format differs from canonical.
	// This enables lossless same-provider roundtrip (Decision 15).
	sourceExt := filepath.Ext(file.Path)
	hasSource := false
	if sourceExt != "" && sourceExt != ".md" {
		sourceDir := filepath.Join(destDir, ".source")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			return destDir, fmt.Errorf("creating .source/ directory: %w", err)
		}
		originalDest := filepath.Join(sourceDir, filepath.Base(file.Path))
		if err := os.WriteFile(originalDest, raw, 0644); err != nil {
			return destDir, fmt.Errorf("writing .source/ copy: %w", err)
		}
		hasSource = true
	}

	// Write .syllago.yaml with add metadata.
	now := time.Now().UTC()
	ver := version
	if ver == "" {
		ver = "syllago"
	}
	sourceFormatExt := strings.TrimPrefix(filepath.Ext(file.Path), ".")
	meta := &metadata.Meta{
		ID:             metadata.NewID(),
		Name:           name,
		Type:           string(ct),
		SourceProvider: sourceProvider,
		SourceFormat:   sourceFormatExt,
		SourceType:     "provider",
		HasSource:      hasSource,
		AddedAt:        &now,
		AddedBy:        ver,
	}
	if err := metadata.Save(destDir, meta); err != nil {
		// Non-fatal: warn but don't fail the add operation.
		fmt.Fprintf(output.ErrWriter, "  warning: could not write metadata for %s: %s\n", name, err)
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
	case catalog.Prompts:
		return "PROMPT.md"
	case catalog.MCP:
		return "mcp.json"
	default:
		return name + ext
	}
}

func printAddDiscoveryReport(report parse.DiscoveryReport) {
	fmt.Fprintf(output.Writer, "Add from %s:\n", report.Provider)
	total := 0
	for ct, count := range report.Counts {
		if count > 0 {
			fmt.Fprintf(output.Writer, "  %s: %d file(s)\n", ct.Label(), count)
			total += count
		}
	}
	if total == 0 {
		fmt.Fprintln(output.Writer, "  No content found.")
		printAddDiscoveryDiagnostics(report)
	}
	if len(report.Unclassified) > 0 {
		fmt.Fprintf(output.Writer, "  %d file(s) couldn't be classified.\n", len(report.Unclassified))
	}
}

// runAddHooks handles "syllago add --type hooks". It reads settings.json
// for the given provider, splits it into individual hook groups, filters by
// --exclude, and either prints a preview or writes each hook to library.
func runAddHooks(root, fromSlug string, previewOnly bool, exclude []string, force bool, scope string, resolver *config.PathResolver) error {
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
		if err := addHooksFromLocation(fromSlug, loc, previewOnly, excludeSet, force); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to add hooks from %s: %v\n", loc.Path, err)
		}
	}
	return nil
}

// addHooksFromLocation reads a single settings.json, splits it into hooks,
// and either previews or writes them.
func addHooksFromLocation(fromSlug string, loc installer.SettingsLocation, previewOnly bool, excludeSet map[string]bool, force bool) error {
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
		fmt.Fprintf(output.Writer, "\n%d hooks would be added.\n", len(filtered))
		return nil
	}

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	count := 0
	for _, hook := range filtered {
		name := converter.DeriveHookName(hook)
		itemDir := filepath.Join(globalDir, string(catalog.Hooks), fromSlug, name)

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
			SourceType:     "provider",
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
	fmt.Fprintf(output.Writer, "\nAdded %d hooks to library.\n", count)
	return nil
}

// printAddDiscoveryDiagnostics explains why no content was found by showing
// which types aren't supported and which paths were searched but empty.
func printAddDiscoveryDiagnostics(report parse.DiscoveryReport) {
	// Show unsupported types so the user knows they can't add those.
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
