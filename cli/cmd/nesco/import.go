package main

import (
	"fmt"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/parse"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Read existing AI tool configs into canonical model",
	Long:  "Discovers and parses provider-specific content files. Read-only — nothing is written to disk.",
	RunE:  runImport,
}

func init() {
	importCmd.Flags().String("from", "", "Provider to import from (required)")
	importCmd.MarkFlagRequired("from")
	importCmd.Flags().String("type", "", "Limit to a single content type (e.g., rules, hooks, mcp)")
	importCmd.Flags().String("name", "", "Filter to items whose path contains this substring (case-insensitive)")
	importCmd.Flags().Bool("preview", false, "Show discovery report without parsing")
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
		output.PrintError(1, "unknown provider: "+fromSlug, "Available: claude-code, cursor, windsurf, codex, gemini-cli")
		return fmt.Errorf("unknown provider: %s", fromSlug)
	}

	typeFilter, _ := cmd.Flags().GetString("type")
	nameFilter, _ := cmd.Flags().GetString("name")
	preview, _ := cmd.Flags().GetBool("preview")

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

	if output.JSON {
		output.Print(result)
	} else {
		printDiscoveryReport(report)
		fmt.Printf("\nParsed %d sections from %d files.\n", len(result.Sections), len(report.Files))
	}

	return nil
}

func printDiscoveryReport(report parse.DiscoveryReport) {
	fmt.Printf("Import from %s:\n", report.Provider)
	total := 0
	for ct, count := range report.Counts {
		if count > 0 {
			fmt.Printf("  %s: %d file(s)\n", ct.Label(), count)
			total += count
		}
	}
	if total == 0 {
		fmt.Println("  No content found.")
		printDiscoveryDiagnostics(report)
	}
	if len(report.Unclassified) > 0 {
		fmt.Printf("  %d file(s) couldn't be classified.\n", len(report.Unclassified))
	}
}

// printDiscoveryDiagnostics explains why no content was found by showing
// which types aren't supported and which paths were searched but empty.
func printDiscoveryDiagnostics(report parse.DiscoveryReport) {
	// Show unsupported types so the user knows they can't import those.
	for _, ct := range report.Unsupported {
		fmt.Printf("  Note: %s is not supported for %s\n", ct.Label(), report.Provider)
	}
	// Show searched paths for supported types that came back empty.
	for ct, paths := range report.SearchedPaths {
		if report.Counts[ct] == 0 {
			fmt.Printf("  No %s found in %s. Searched: %s\n", ct.Label(), report.Provider, strings.Join(paths, ", "))
		}
	}
}
