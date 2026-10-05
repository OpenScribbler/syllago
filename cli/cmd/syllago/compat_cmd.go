package main

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/spf13/cobra"
)

// compatEntry is the JSON-serializable output for one provider row.
type compatEntry struct {
	Provider  string   `json:"provider"`
	Supported bool     `json:"supported"`
	Warnings  []string `json:"warnings,omitempty"`
}

// compatOutput is the top-level JSON output for syllago compat.
type compatOutput struct {
	Name    string        `json:"name"`
	Type    string        `json:"type"`
	Entries []compatEntry `json:"entries"`
}

var compatCmd = &cobra.Command{
	Use:   "compat <name>",
	Short: "Show provider compatibility matrix for a content item",
	Long: `Analyzes a library item and shows which providers support it,
what warnings arise during conversion, and which providers cannot
handle the content type at all.

For each provider, syllago attempts the full canonicalize-then-render
pipeline and reports the result.`,
	Example: `  # Show compatibility for a skill
  syllago compat my-skill

  # JSON output for scripting
  syllago compat my-skill --json`,
	Args: cobra.ExactArgs(1),
	RunE: runCompat,
}

func init() {
	rootCmd.AddCommand(compatCmd)
}

func runCompat(cmd *cobra.Command, args []string) error {
	name := args[0]

	cat, err := scanGlobalLibrary()
	if err != nil {
		return err
	}

	var item *catalog.ContentItem
	for i := range cat.Items {
		if cat.Items[i].Name == name {
			item = &cat.Items[i]
			break
		}
	}
	if item == nil {
		return output.NewStructuredError(output.ErrItemNotFound, fmt.Sprintf("no item named %q in your library", name), "Run 'syllago list' to show all library items")
	}

	report, err := converter.CompatReport(*item)
	if err != nil {
		return convertItemError(err, *item, provider.Provider{})
	}

	result := compatOutput{
		Name: item.Name,
		Type: item.Type.Label(),
	}
	for _, row := range report {
		result.Entries = append(result.Entries, compatEntry{
			Provider:  row.Provider.Slug,
			Supported: row.Supported,
			Warnings:  row.Warnings,
		})
	}

	if output.JSON {
		output.Print(result)
		return nil
	}

	// Plain text table output.
	w := tabwriter.NewWriter(output.Writer, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Provider\tSupported\tWarnings\n")
	for _, e := range result.Entries {
		symbol := "✓"
		if !e.Supported {
			symbol = "✗"
		}
		warnings := strings.Join(e.Warnings, "; ")
		fmt.Fprintf(w, "%s\t%s\t%s\n", e.Provider, symbol, warnings)
	}
	_ = w.Flush()

	return nil
}
