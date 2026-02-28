package main

import (
	"fmt"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List content items in the catalog",
	Long: `Show a quick inventory of all content without launching the TUI.

By default, lists all content grouped by type. Use flags to filter:
  nesco list                    All content, grouped by type
  nesco list --source local     Only local items
  nesco list --type skills      Only skills
  nesco list --json             JSON output`,
	RunE: runList,
}

func init() {
	listCmd.Flags().String("source", "all", "Filter by source: local, shared, registry, builtin, all")
	listCmd.Flags().String("type", "", "Filter to one content type (e.g., skills, rules)")
	rootCmd.AddCommand(listCmd)
}

// listResult is the JSON-serializable output for nesco list.
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
}

func runList(cmd *cobra.Command, args []string) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find nesco repo: %w", err)
	}

	sourceFilter, _ := cmd.Flags().GetString("source")
	typeFilter, _ := cmd.Flags().GetString("type")

	projectRoot, _ := findProjectRoot()
	if projectRoot == "" {
		projectRoot = root
	}
	cat, err := catalog.Scan(root, projectRoot)
	if err != nil {
		return fmt.Errorf("scanning catalog: %w", err)
	}

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
			items = append(items, listItem{
				Name:        item.Name,
				Source:      sourceLabel(item),
				Description: item.Description,
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
			// Pad name and source for alignment.
			fmt.Fprintf(output.Writer, "  %-18s [%-8s] %s\n",
				item.Name, item.Source, item.Description)
		}
	}

	return nil
}

// sourceLabel returns a human-readable source tag for display.
func sourceLabel(item catalog.ContentItem) string {
	switch {
	case item.IsBuiltin():
		return "builtin"
	case item.Registry != "":
		return "registry"
	case item.Local:
		return "local"
	default:
		return "shared"
	}
}
