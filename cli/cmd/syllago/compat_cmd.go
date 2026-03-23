package main

import (
	"fmt"
	"os"
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

	globalDir := catalog.GlobalContentDir()
	cat, err := catalog.ScanWithGlobalAndRegistries(globalDir, globalDir, nil)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library failed", "Check that ~/.syllago/content/ exists and is readable", err.Error())
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

	conv := converter.For(item.Type)

	// Determine the source provider for canonicalization.
	srcProvider := ""
	if item.Meta != nil {
		srcProvider = item.Meta.SourceProvider
	}
	if srcProvider == "" && item.Provider != "" {
		srcProvider = item.Provider
	}

	// Pre-read and canonicalize content once (if a converter exists).
	var canonical *converter.Result
	if conv != nil {
		contentFile := converter.ResolveContentFile(*item)
		if contentFile != "" {
			raw, readErr := os.ReadFile(contentFile)
			if readErr == nil {
				canonical, _ = conv.Canonicalize(raw, srcProvider)
			}
		}
	}

	result := compatOutput{
		Name: item.Name,
		Type: item.Type.Label(),
	}

	for _, prov := range provider.AllProviders {
		entry := compatEntry{
			Provider: prov.Slug,
		}

		// Check if the provider supports this content type at all.
		if prov.SupportsType == nil || !prov.SupportsType(item.Type) {
			entry.Supported = false
			entry.Warnings = []string{item.Type.Label() + " not supported"}
			result.Entries = append(result.Entries, entry)
			continue
		}

		// No converter registered for this type — supported but no conversion needed.
		if conv == nil || canonical == nil {
			entry.Supported = true
			result.Entries = append(result.Entries, entry)
			continue
		}

		// Attempt render to this provider's format.
		rendered, renderErr := conv.Render(canonical.Content, prov)
		if renderErr != nil {
			entry.Supported = false
			entry.Warnings = []string{renderErr.Error()}
			result.Entries = append(result.Entries, entry)
			continue
		}

		if rendered.Content == nil {
			entry.Supported = false
			entry.Warnings = []string{"conversion produced no output"}
			result.Entries = append(result.Entries, entry)
			continue
		}

		entry.Supported = true
		entry.Warnings = rendered.Warnings
		result.Entries = append(result.Entries, entry)
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
