package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/converter"
	"github.com/OpenScribbler/nesco/cli/internal/metadata"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/OpenScribbler/nesco/cli/internal/parse"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Bring content into nesco from a provider, path, or git URL",
	Long: `Discovers content from a provider and imports it into local/.

Nesco handles format conversion automatically. Once imported, content can be
exported to any supported provider with "nesco export --to <provider>".

Examples:
  nesco import --from claude-code                  Import all content from Claude Code
  nesco import --from claude-code --type skills    Import only skills
  nesco import --from cursor --name my-rule        Import a specific rule by name
  nesco import --from claude-code --preview        Preview what would be imported (read-only)
  nesco import --from claude-code --dry-run        Show what would be written without writing

After import, use "nesco export" to install content into other providers,
or browse in the TUI with "nesco".`,
	RunE: runImport,
}

func init() {
	importCmd.Flags().String("from", "", "Provider to import from (required)")
	importCmd.MarkFlagRequired("from")
	importCmd.Flags().String("type", "", "Limit to a single content type (e.g., rules, hooks, mcp)")
	importCmd.Flags().String("name", "", "Filter to items whose path contains this substring (case-insensitive)")
	importCmd.Flags().Bool("preview", false, "Show discovery report without parsing")
	importCmd.Flags().Bool("dry-run", false, "Show what would be written without actually writing")
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
		output.PrintError(1, "unknown provider: "+fromSlug, "Available: "+strings.Join(slugs, ", "))
		return fmt.Errorf("unknown provider: %s", fromSlug)
	}

	typeFilter, _ := cmd.Flags().GetString("type")
	nameFilter, _ := cmd.Flags().GetString("name")
	preview, _ := cmd.Flags().GetBool("preview")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	report := parse.Discover(*prov, root)

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
		fmt.Fprintf(output.Writer, "\nImported %d file(s) to local/.\n", written)
	} else {
		printDiscoveryReport(report)
		fmt.Fprintf(output.Writer, "\n[dry-run] Would import %d file(s) to local/.\n", written)
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

	// Write .nesco.yaml with import metadata.
	now := time.Now().UTC()
	sourceExt := strings.TrimPrefix(filepath.Ext(file.Path), ".")
	meta := &metadata.Meta{
		ID:             metadata.NewID(),
		Name:           name,
		Type:           string(ct),
		ImportedAt:     &now,
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
	case catalog.Prompts:
		return "PROMPT.md"
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
