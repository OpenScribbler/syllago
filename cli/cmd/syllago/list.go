package main

import (
	"fmt"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
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

func init() {
	listCmd.Flags().String("source", "all", "Filter by source: library, shared, registry, builtin, all")
	listCmd.Flags().String("type", "", "Filter to one content type (e.g., skills, rules)")
	rootCmd.AddCommand(listCmd)
}

// listResult is the JSON-serializable output for syllago list.
type listResult struct {
	Groups []listGroup `json:"groups"`
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

func runList(cmd *cobra.Command, args []string) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogNotFound, "could not find syllago repo", "Run 'syllago init' to set up a content repository", err.Error())
	}

	sourceFilter, _ := cmd.Flags().GetString("source")
	typeFilter, _ := cmd.Flags().GetString("type")

	projectRoot, _ := findProjectRoot()
	if projectRoot == "" {
		projectRoot = root
	}
	scan, err := moat.LoadAndScan(root, projectRoot, time.Now())
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning catalog failed", "Check that the content directory exists and is readable", err.Error())
	}
	cat := scan.Catalog
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

	totalItems := 0
	for _, g := range result.Groups {
		totalItems += g.Count
	}
	telemetry.Enrich("source_filter", sourceFilter)
	telemetry.Enrich("content_type", typeFilter)
	telemetry.Enrich("item_count", totalItems)

	if output.JSON {
		output.Print(result)
		return nil
	}

	if len(result.Groups) == 0 {
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

	return nil
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
