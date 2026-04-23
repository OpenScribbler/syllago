package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh [name]",
	Short: "Check for and apply content updates from registries",
	Long: `Compares installed content versions against their source registries
and updates items where a newer version is available.

Version comparison uses semver from .syllago.yaml metadata.
Items installed from local projects (not registries) are skipped.`,
	Example: `  # Refresh a specific item
  syllago refresh my-skill

  # Check and refresh all installed items
  syllago refresh --all

  # Dry-run: show what would be updated
  syllago refresh --all --dry-run

  # Only refresh items from a specific registry
  syllago refresh --all --registry team-rules`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRefresh,
}

func init() {
	refreshCmd.Flags().Bool("all", false, "Refresh all installed items")
	refreshCmd.Flags().Bool("dry-run", false, "Show what would be updated without changing anything")
	refreshCmd.Flags().String("registry", "", "Only refresh items from this registry")
	rootCmd.AddCommand(refreshCmd)
}

func runRefresh(cmd *cobra.Command, args []string) error {
	// TODO: implement content refresh logic
	// 1. Load installed.json
	// 2. For each item, sync registry and compare versions
	// 3. Queue and apply updates with snapshot rollback
	return fmt.Errorf("content refresh is not yet implemented")
}
