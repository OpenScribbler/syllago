package main

import (
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/rollback"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

type rollbackResult struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	Registry            string `json:"registry"`
	RestoredSourceSHA   string `json:"restored_source_sha,omitempty"`
	RestoredFromCopy    bool   `json:"restored_from_copy"`
	PlacementsReapplied int    `json:"placements_reapplied"`
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback <name>",
	Short: "Restore the previous installed library version",
	Long: `Restores a registry-sourced library item to its one-step rollback point.

Rollback is available after an update replaces an installed item. It works for
pinned items because rollback is an explicit user action.`,
	Example: `  # Roll back a skill to the previous version
  syllago rollback my-skill

  # Disambiguate by type
  syllago rollback shared-context --type skills

  # Preview the rollback source without changing files
  syllago rollback my-skill --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runRollback,
}

func init() {
	rollbackCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
	rollbackCmd.Flags().Bool("dry-run", false, "Show what would happen without making changes")
	rootCmd.AddCommand(rollbackCmd)
}

func runRollback(cmd *cobra.Command, args []string) error {
	name := args[0]
	typeFilter, _ := cmd.Flags().GetString("type")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	item, err := resolveRollbackLibraryItem(name, typeFilter)
	if err != nil {
		return err
	}
	telemetry.Enrich("content_type", string(item.Type))
	telemetry.Enrich("dry_run", dryRun)

	plan, err := rollback.PlanFor(*item)
	if err != nil {
		return err
	}

	result := rollbackResult{
		Name:     item.Name,
		Type:     string(item.Type),
		Registry: plan.Coord.Registry,
	}
	if plan.FromCopy {
		result.RestoredFromCopy = true
	} else {
		result.RestoredSourceSHA = plan.Prev.SourceSHA
	}

	if dryRun {
		if output.JSON {
			output.Print(result)
			return nil
		}
		printRollbackDryRunPlan(*item, &plan.Prev)
		return nil
	}

	if err := rollback.Restore(plan, version); err != nil {
		return err
	}

	var reappliedPlacements []installstore.Placement
	restoredItem, err := resolveRollbackLibraryItem(item.Name, string(item.Type))
	if err != nil {
		fmt.Fprintf(output.ErrWriter, "warning: could not reload restored item for placement re-apply: %v\n", err)
	} else {
		root, rootErr := findProjectRoot()
		var warnings []string
		reappliedPlacements, warnings = rollback.ReapplyPlacements(*restoredItem, plan.Placements, rollback.Options{
			ProjectRoot:    root,
			ProjectRootErr: rootErr,
		})
		for _, w := range warnings {
			fmt.Fprintf(output.ErrWriter, "warning: %s\n", w)
		}
		result.PlacementsReapplied = len(reappliedPlacements)
	}

	if output.JSON {
		output.Print(result)
		return nil
	}
	if output.Quiet {
		return nil
	}
	if result.RestoredFromCopy {
		fmt.Fprintf(output.Writer, "Rolled back %s/%s to previous version\n", item.Type, item.Name)
	} else {
		fmt.Fprintf(output.Writer, "Rolled back %s/%s to %s\n", item.Type, item.Name, rollback.ShortSHA(plan.Prev.SourceSHA))
	}
	for _, pl := range reappliedPlacements {
		fmt.Fprintf(output.Writer, "Re-applied %s placement for %s\n", pl.Mechanism, pl.Provider)
	}
	return nil
}

func resolveRollbackLibraryItem(name, typeFilter string) (*catalog.ContentItem, error) {
	emptyRoot, err := os.MkdirTemp("", "syllago-rollback-*")
	if err != nil {
		return nil, output.NewStructuredErrorDetail(output.ErrSystemIO, "creating temp dir failed", "Check filesystem permissions and disk space", err.Error())
	}
	defer func() { _ = os.RemoveAll(emptyRoot) }()

	cat, err := catalog.ScanWithGlobalAndRegistries(emptyRoot, emptyRoot, nil)
	if err != nil {
		return nil, output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library failed", "Check that ~/.syllago/content/ exists and is readable", err.Error())
	}
	return findLibraryItem(cat, name, typeFilter)
}

func printRollbackDryRunPlan(item catalog.ContentItem, prev *installstore.PreviousVersion) {
	if output.Quiet && !output.JSON {
		return
	}
	if prev.CopyPath != "" {
		when := "unknown date"
		if !prev.ReplacedAt.IsZero() {
			when = prev.ReplacedAt.Format("2006-01-02")
		}
		fmt.Fprintf(output.Writer, "would roll back %s/%s to saved copy from %s\n", item.Type, item.Name, when)
		return
	}
	fmt.Fprintf(output.Writer, "would roll back %s/%s to %s\n", item.Type, item.Name, rollback.ShortSHA(prev.SourceSHA))
}
