package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

// convertResult is the JSON-serializable output for syllago convert.
type convertResult struct {
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	Output   string   `json:"output,omitempty"`   // path if --output was used; empty for stdout
	Warnings []string `json:"warnings,omitempty"` // portability warnings from conversion
}

var convertCmd = &cobra.Command{
	Use:   "convert <name>",
	Short: "Convert library content to a provider format",
	Long: `Renders a library item to a target provider's format without installing it.
Output goes to stdout by default, or to a file with --output.

No state changes are made — this is purely for ad-hoc sharing.`,
	Example: `  # Convert a skill to Cursor format (stdout)
  syllago convert my-skill --to cursor

  # Convert and save to a file
  syllago convert my-rule --to windsurf --output ./windsurf-rule.md

  # Batch-canonicalize a directory of hooks
  syllago convert --batch ./hooks/ --from claude-code --to canonical

  # Batch dry-run
  syllago convert --batch ./hooks/ --from claude-code --to canonical --dry-run`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConvert,
}

func init() {
	convertCmd.Flags().String("to", "", "Target provider (required)")
	convertCmd.MarkFlagRequired("to")
	convertCmd.Flags().StringP("output", "o", "", "Write output to this file path (default: stdout)")
	convertCmd.Flags().String("batch", "", "Directory of hook files to batch-canonicalize (mutual exclusive with <name>)")
	convertCmd.Flags().String("from", "", "Source provider slug (required with --batch)")
	convertCmd.Flags().Bool("dry-run", false, "Show what would be converted without writing files")
	rootCmd.AddCommand(convertCmd)
}

func runConvert(cmd *cobra.Command, args []string) error {
	name := args[0]
	toSlug, _ := cmd.Flags().GetString("to")
	outputPath, _ := cmd.Flags().GetString("output")

	prov := findProviderBySlug(toSlug)
	if prov == nil {
		slugs := providerSlugs()
		return fmt.Errorf("unknown provider: %s\n  Available: %s", toSlug, strings.Join(slugs, ", "))
	}

	globalDir := catalog.GlobalContentDir()
	cat, err := catalog.ScanWithGlobalAndRegistries(globalDir, globalDir, nil)
	if err != nil {
		return fmt.Errorf("scanning library: %w", err)
	}

	var item *catalog.ContentItem
	for i := range cat.Items {
		if cat.Items[i].Name == name {
			item = &cat.Items[i]
			break
		}
	}
	if item == nil {
		return fmt.Errorf("no item named %q in your library.\n  Hint: syllago list    (show all library items)", name)
	}

	conv := converter.For(item.Type)
	if conv == nil {
		return fmt.Errorf("%s does not support format conversion", item.Type.Label())
	}

	contentFile := converter.ResolveContentFile(*item)
	if contentFile == "" {
		return fmt.Errorf("cannot locate content file for %s", name)
	}
	raw, err := os.ReadFile(contentFile)
	if err != nil {
		return fmt.Errorf("reading content: %w", err)
	}

	srcProvider := ""
	if item.Meta != nil {
		srcProvider = item.Meta.SourceProvider
	}
	if srcProvider == "" && item.Provider != "" {
		srcProvider = item.Provider
	}

	canonical, err := conv.Canonicalize(raw, srcProvider)
	if err != nil {
		return fmt.Errorf("canonicalizing content: %w", err)
	}

	rendered, err := conv.Render(canonical.Content, *prov)
	if err != nil {
		return fmt.Errorf("rendering to %s format: %w", prov.Name, err)
	}
	if rendered.Content == nil {
		return fmt.Errorf("%s is not compatible with %s format", name, prov.Name)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, rendered.Content, 0644); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
		if output.JSON {
			output.Print(convertResult{Name: name, Provider: prov.Slug, Output: outputPath, Warnings: rendered.Warnings})
		} else if !output.Quiet {
			fmt.Fprintf(output.Writer, "Rendered %s as %s format to %s\n", name, prov.Name, outputPath)
		}
	} else {
		if output.JSON {
			output.Print(convertResult{Name: name, Provider: prov.Slug, Warnings: rendered.Warnings})
		}
		os.Stdout.Write(rendered.Content)
	}

	// Surface portability warnings to stderr so they're visible even with stdout output.
	if !output.JSON && !output.Quiet && len(rendered.Warnings) > 0 {
		fmt.Fprintf(output.ErrWriter, "\n  Portability warnings:\n")
		for _, w := range rendered.Warnings {
			fmt.Fprintf(output.ErrWriter, "    - %s\n", w)
		}
		fmt.Fprintln(output.ErrWriter)
	}

	return nil
}
