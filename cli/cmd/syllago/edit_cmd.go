package main

import (
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

// editResult is the JSON-serializable output for syllago edit.
type editResult struct {
	Item        string `json:"item"`
	Type        string `json:"type"`
	OldName     string `json:"old_name,omitempty"`
	NewName     string `json:"new_name,omitempty"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path"`
}

var editCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit a content item's display name or description",
	Long: `Updates display name and/or description stored in the item's .syllago.yaml metadata.

Only the metadata file is changed — the directory name, install paths, and all
references remain exactly as they are.

The display name and description appear in the TUI, list output, and anywhere
items are presented to users.`,
	Example: `  # Set a display name for a hook
  syllago edit my-hook --name "Pre-commit Linter"

  # Set a description
  syllago edit my-hook --description "Runs ESLint before every commit"

  # Update both at once
  syllago edit my-hook --name "Pre-commit Linter" --description "Runs ESLint before every commit"

  # Disambiguate by type when name exists in multiple types
  syllago edit my-item --name "Better Name" --type hooks`,
	Args: cobra.ExactArgs(1),
	RunE: runEdit,
}

func init() {
	editCmd.Flags().String("name", "", "New display name for the item")
	editCmd.Flags().String("description", "", "New description for the item")
	editCmd.Flags().String("type", "", "Content type to disambiguate (skills, hooks, mcp, etc.)")
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	newName, _ := cmd.Flags().GetString("name")
	newDescription, _ := cmd.Flags().GetString("description")

	if newName == "" && newDescription == "" {
		return output.NewStructuredError(output.ErrInputMissing,
			"at least one of --name or --description is required",
			"Example: syllago edit my-hook --name \"Better Name\"")
	}

	itemName := args[0]
	typeFilter, _ := cmd.Flags().GetString("type")

	// Scan library items (global content only, no project scope)
	emptyProjectRoot, err := os.MkdirTemp("", "syllago-edit-*")
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

	result := editResult{
		Item: itemName,
		Type: string(item.Type),
		Path: item.Path,
	}

	if newName != "" {
		result.OldName = meta.Name
		result.NewName = newName
		meta.Name = newName
	}
	if newDescription != "" {
		result.Description = newDescription
		meta.Description = newDescription
	}

	if err := metadata.Save(item.Path, meta); err != nil {
		return fmt.Errorf("saving metadata: %w", err)
	}

	if output.JSON {
		output.Print(result)
	} else if !output.Quiet {
		if newName != "" {
			fmt.Fprintf(output.Writer, "Updated %s %q: name %q → %q\n", item.Type, itemName, result.OldName, newName)
		}
		if newDescription != "" {
			fmt.Fprintf(output.Writer, "Updated %s %q: description set\n", item.Type, itemName)
		}
	}

	return nil
}
