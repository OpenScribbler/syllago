package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/installer"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/provider"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export items from my-tools/ to a provider's install location",
	Long:  "Copies content from my-tools/ into the target provider's filesystem locations (e.g., ~/.claude/skills).",
	RunE:  runExport,
}

func init() {
	exportCmd.Flags().String("to", "", "Provider slug to export to (required)")
	exportCmd.MarkFlagRequired("to")
	exportCmd.Flags().String("type", "", "Filter to a specific content type (e.g., skills, rules)")
	exportCmd.Flags().String("name", "", "Filter by item name (substring match)")
	rootCmd.AddCommand(exportCmd)
}

type exportResult struct {
	Exported []exportedItem `json:"exported"`
	Skipped  []skippedItem  `json:"skipped,omitempty"`
}

type exportedItem struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Destination string `json:"destination"`
}

type skippedItem struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

func runExport(cmd *cobra.Command, args []string) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find nesco repo: %w", err)
	}

	toSlug, _ := cmd.Flags().GetString("to")
	prov := findProviderBySlug(toSlug)
	if prov == nil {
		slugs := providerSlugs()
		output.PrintError(1, "unknown provider: "+toSlug,
			"Available: "+strings.Join(slugs, ", "))
		return output.SilentError(fmt.Errorf("unknown provider: %s", toSlug))
	}

	typeFilter, _ := cmd.Flags().GetString("type")
	nameFilter, _ := cmd.Flags().GetString("name")

	// Scan the catalog to find local (my-tools/) items.
	cat, err := catalog.Scan(root)
	if err != nil {
		return fmt.Errorf("scanning catalog: %w", err)
	}

	// Collect only local items, applying filters.
	var items []catalog.ContentItem
	for _, item := range cat.Items {
		if !item.Local {
			continue
		}
		if typeFilter != "" && string(item.Type) != typeFilter {
			continue
		}
		if nameFilter != "" && !strings.Contains(item.Name, nameFilter) {
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		msg := "no items found in my-tools/"
		if typeFilter != "" || nameFilter != "" {
			msg += " matching filters"
		}
		fmt.Fprintln(output.ErrWriter, msg)
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	result := exportResult{}

	for _, item := range items {
		// Check if provider supports this type via SupportsType.
		if prov.SupportsType != nil && !prov.SupportsType(item.Type) {
			skip := skippedItem{
				Name:   item.Name,
				Type:   string(item.Type),
				Reason: fmt.Sprintf("%s does not support %s", prov.Name, item.Type.Label()),
			}
			result.Skipped = append(result.Skipped, skip)
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): %s does not support %s\n",
					item.Name, item.Type.Label(), prov.Name, item.Type.Label())
			}
			continue
		}

		// Check for JSON merge sentinel — these types require config-file
		// merging rather than filesystem copy. Skip with a warning.
		installDir := prov.InstallDir(homeDir, item.Type)
		if installDir == provider.JSONMergeSentinel {
			skip := skippedItem{
				Name:   item.Name,
				Type:   string(item.Type),
				Reason: fmt.Sprintf("%s for %s requires JSON merge (not supported by export)", item.Type.Label(), prov.Name),
			}
			result.Skipped = append(result.Skipped, skip)
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): requires JSON merge for %s (use the TUI to install)\n",
					item.Name, item.Type.Label(), prov.Name)
			}
			continue
		}

		if installDir == "" {
			skip := skippedItem{
				Name:   item.Name,
				Type:   string(item.Type),
				Reason: fmt.Sprintf("%s does not support %s", prov.Name, item.Type.Label()),
			}
			result.Skipped = append(result.Skipped, skip)
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): %s does not support %s\n",
					item.Name, item.Type.Label(), prov.Name, item.Type.Label())
			}
			continue
		}

		// Determine destination: installDir/<item-name>
		dest := filepath.Join(installDir, item.Name)

		if err := os.MkdirAll(installDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", installDir, err)
		}

		if err := installer.CopyContent(item.Path, dest); err != nil {
			return fmt.Errorf("copying %s: %w", item.Name, err)
		}

		result.Exported = append(result.Exported, exportedItem{
			Name:        item.Name,
			Type:        string(item.Type),
			Destination: dest,
		})

		if !output.JSON {
			fmt.Fprintf(output.Writer, "Exported %s to %s\n", item.Name, dest)
		}
	}

	if output.JSON {
		output.Print(result)
	} else if len(result.Exported) == 0 && len(result.Skipped) > 0 {
		fmt.Fprintln(output.ErrWriter, "No items were exported (all skipped).")
	}

	return nil
}

