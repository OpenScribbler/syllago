package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var updateContentCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Check for and apply content updates from registries",
	Long: `Compares installed content versions against their source registries
and updates items where a newer version is available.

Version comparison uses semver from .syllago.yaml metadata.
Items installed from local projects (not registries) are skipped.`,
	Example: `  # Update a specific item
  syllago update my-skill

  # Check and update all installed items
  syllago update --all

  # Dry-run: show what would be updated
  syllago update --all --dry-run

  # Only update items from a specific registry
  syllago update --all --registry team-rules`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdateContent,
}

func init() {
	updateContentCmd.Flags().Bool("all", false, "Update all installed items")
	updateContentCmd.Flags().Bool("dry-run", false, "Show what would be updated without changing anything")
	updateContentCmd.Flags().String("registry", "", "Only update items from this registry")
	rootCmd.AddCommand(updateContentCmd)
}

func runUpdateContent(cmd *cobra.Command, args []string) error {
	// TODO: implement content update logic
	// 1. Load installed.json
	// 2. For each item, sync registry and compare versions
	// 3. Queue and apply updates with snapshot rollback
	return fmt.Errorf("content updates are not yet implemented")
}
