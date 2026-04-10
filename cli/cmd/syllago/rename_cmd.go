package main

import (
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

// renameResult is the JSON-serializable output for syllago rename.
type renameResult struct {
	Item    string `json:"item"`
	Type    string `json:"type"`
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
	Path    string `json:"path"`
}

var renameCmd = &cobra.Command{
	Use:   "rename <name>",
	Short: "Set a human-readable display name for a content item",
	Long: `Updates the display name stored in the item's .syllago.yaml metadata.
This is especially useful for hooks and MCP servers where the directory
name alone isn't descriptive enough.

The display name is shown in the TUI, list output, and anywhere items
are presented to users. The directory name (used for install paths and
references) is not changed.`,
	Example: `  # Name a hook
  syllago rename my-hook --name "Pre-commit Linter"

  # Name an MCP server
  syllago rename postgres --name "Local Postgres Dev" --type mcp

  # Disambiguate by type when name exists in multiple types
  syllago rename my-item --name "Better Name" --type hooks`,
	Args: cobra.ExactArgs(1),
	RunE: runRename,
}

func init() {
	renameCmd.Flags().String("name", "", "The new display name (required)")
	renameCmd.Flags().String("type", "", "Content type to disambiguate (skills, hooks, mcp, etc.)")
	_ = renameCmd.MarkFlagRequired("name")
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	itemName := args[0]
	newName, _ := cmd.Flags().GetString("name")
	typeFilter, _ := cmd.Flags().GetString("type")

	// Scan library items (global content only, no project scope)
	emptyProjectRoot, err := os.MkdirTemp("", "syllago-rename-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(emptyProjectRoot) }()

	cat, err := catalog.ScanWithGlobalAndRegistries(emptyProjectRoot, emptyProjectRoot, nil)
	if err != nil {
		return fmt.Errorf("scanning library: %w", err)
	}

	// Find the item
	var matches []catalog.ContentItem
	for _, item := range cat.Items {
		if item.Name != itemName {
			continue
		}
		if typeFilter != "" && string(item.Type) != typeFilter {
			continue
		}
		matches = append(matches, item)
	}

	if len(matches) == 0 {
		return output.NewStructuredError(output.ErrItemNotFound,
			fmt.Sprintf("no item named %q found in your library", itemName),
			"Run 'syllago list' to show all library items")
	}
	if len(matches) > 1 {
		var types []string
		for _, m := range matches {
			types = append(types, string(m.Type))
		}
		return output.NewStructuredError(output.ErrItemAmbiguous,
			fmt.Sprintf("item %q exists in multiple types: %v", itemName, types),
			"Use --type to disambiguate (e.g., --type hooks)")
	}

	item := matches[0]

	// Load or create metadata
	meta, loadErr := metadata.Load(item.Path)
	if loadErr != nil {
		return fmt.Errorf("loading metadata: %w", loadErr)
	}
	if meta == nil {
		meta = &metadata.Meta{
			Name: item.Name,
			Type: string(item.Type),
		}
	}

	oldName := meta.Name
	meta.Name = newName

	if err := metadata.Save(item.Path, meta); err != nil {
		return fmt.Errorf("saving metadata: %w", err)
	}

	if output.JSON {
		output.Print(renameResult{
			Item:    itemName,
			Type:    string(item.Type),
			OldName: oldName,
			NewName: newName,
			Path:    item.Path,
		})
	} else if !output.Quiet {
		fmt.Fprintf(output.Writer, "Renamed %s %q → %q\n", item.Type, itemName, newName)
	}

	return nil
}
