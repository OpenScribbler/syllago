package main

import (
	"fmt"

	"github.com/OpenScribbler/nesco/cli/internal/config"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/OpenScribbler/nesco/cli/internal/registry"
	"github.com/spf13/cobra"
)

var syncAndExportCmd = &cobra.Command{
	Use:   "sync-and-export",
	Short: "Sync registries then export content to a provider",
	Long: `Convenience command that syncs all registries then exports.

Equivalent to running:
  nesco registry sync && nesco export --to <provider>

This is useful in CI/CD or automation where you want a single command
to ensure registries are up-to-date before exporting.

Examples:
  nesco sync-and-export --to cursor
  nesco sync-and-export --to all --type skills
  nesco sync-and-export --to kiro --source registry`,
	RunE: runSyncAndExport,
}

func init() {
	syncAndExportCmd.Flags().String("to", "", "Provider slug to export to, or \"all\" for every provider (required)")
	syncAndExportCmd.MarkFlagRequired("to")
	syncAndExportCmd.Flags().String("type", "", "Filter to a specific content type (e.g., skills, rules)")
	syncAndExportCmd.Flags().String("name", "", "Filter by item name (substring match)")
	syncAndExportCmd.Flags().String("source", "local", "Which items to export: local (default), shared, registry, builtin, all")
	syncAndExportCmd.Flags().String("llm-hooks", "skip", "How to handle LLM-evaluated hooks: skip (drop with warning) or generate (create wrapper scripts)")
	rootCmd.AddCommand(syncAndExportCmd)
}

func runSyncAndExport(cmd *cobra.Command, args []string) error {
	// Find project root and load config to get registry list.
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("could not find project root: %w", err)
	}

	cfg, err := config.Load(root)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Sync registries if any are configured.
	if len(cfg.Registries) > 0 {
		names := make([]string, len(cfg.Registries))
		for i, r := range cfg.Registries {
			names[i] = r.Name
		}

		if !output.JSON {
			fmt.Fprintf(output.Writer, "Syncing %d registries...\n", len(names))
		}

		results := registry.SyncAll(names)
		for _, res := range results {
			if res.Err != nil {
				return fmt.Errorf("registry sync failed for %q: %w", res.Name, res.Err)
			}
			if !output.JSON {
				fmt.Fprintf(output.Writer, "Synced: %s\n", res.Name)
			}
		}
	}

	// Now run the export. Copy flag values from this command to exportCmd
	// so the existing export logic picks them up.
	flagNames := []string{"to", "type", "name", "source", "llm-hooks"}
	for _, name := range flagNames {
		val, _ := cmd.Flags().GetString(name)
		exportCmd.Flags().Set(name, val)
	}
	defer func() {
		// Reset export flags to defaults so other tests/callers aren't affected.
		exportCmd.Flags().Set("to", "")
		exportCmd.Flags().Set("type", "")
		exportCmd.Flags().Set("name", "")
		exportCmd.Flags().Set("source", "local")
		exportCmd.Flags().Set("llm-hooks", "skip")
	}()

	return runExport(exportCmd, nil)
}
