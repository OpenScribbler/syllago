package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
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

	if item.Meta == nil || item.Meta.SourceRegistry == "" {
		return output.NewStructuredError(
			output.ErrInputInvalid,
			fmt.Sprintf("%s/%s is not registry-sourced", item.Type, item.Name),
			"Rollback needs an item installed from a registry update.",
		)
	}

	storePath, rec, coord, err := loadRollbackRecord(*item)
	if err != nil {
		return err
	}
	prev := rec.Previous
	if prev == nil || prev.SourceSHA == "" && prev.CopyPath == "" {
		return noRollbackDataError(coord)
	}

	result := rollbackResult{
		Name:     item.Name,
		Type:     string(item.Type),
		Registry: coord.Registry,
	}
	if prev.CopyPath != "" {
		result.RestoredFromCopy = true
	} else {
		result.RestoredSourceSHA = prev.SourceSHA
	}

	if dryRun {
		if output.JSON {
			output.Print(result)
			return nil
		}
		printRollbackDryRunPlan(*item, prev)
		return nil
	}

	placements := append([]installstore.Placement(nil), rec.Placements...)

	if prev.CopyPath != "" {
		if err := restoreFromPreviousCopy(rec.LibraryPath, prev.CopyPath); err != nil {
			return err
		}
	} else {
		if err := restoreFromGitPrevious(*item, rec.LibraryPath, coord.Registry, prev.SourceSHA); err != nil {
			return err
		}
	}

	if err := installstore.RecordRollback(storePath, coord, rec.LibraryPath, time.Now()); err != nil {
		return output.NewStructuredErrorDetail(
			output.ErrSystemIO,
			"content was restored but install record rollback failed",
			"Inspect the library item and install record before running another update.",
			err.Error(),
		)
	}

	var reappliedPlacements []installstore.Placement
	restoredItem, err := resolveRollbackLibraryItem(item.Name, string(item.Type))
	if err != nil {
		fmt.Fprintf(output.ErrWriter, "warning: could not reload restored item for placement re-apply: %v\n", err)
	} else {
		reappliedPlacements = reapplyRollbackPlacements(*restoredItem, placements)
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
		fmt.Fprintf(output.Writer, "Rolled back %s/%s to %s\n", item.Type, item.Name, shortSHA(prev.SourceSHA))
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

func loadRollbackRecord(item catalog.ContentItem) (string, *installstore.Record, installstore.Coord, error) {
	coord := installstore.Coord{
		Registry: item.Meta.SourceRegistry,
		Type:     string(item.Type),
		Name:     item.Name,
	}
	storePath, err := installstore.DefaultPath()
	if err != nil {
		return "", nil, coord, output.NewStructuredErrorDetail(output.ErrSystemHomedir, "install record path unavailable", "Set the HOME environment variable", err.Error())
	}
	if _, err := os.Stat(storePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil, coord, noInstallRecordError(coord)
		}
		return "", nil, coord, output.NewStructuredErrorDetail(output.ErrSystemIO, "checking install record store failed", "Check ~/.syllago/installs.json permissions", err.Error())
	}
	store, err := installstore.Load(storePath)
	if err != nil {
		return "", nil, coord, output.NewStructuredErrorDetail(output.ErrSystemIO, "loading install record store failed", "Check ~/.syllago/installs.json syntax and permissions", err.Error())
	}
	rec := store.Find(coord)
	if rec == nil {
		return "", nil, coord, noInstallRecordError(coord)
	}
	return storePath, rec, coord, nil
}

func noInstallRecordError(c installstore.Coord) error {
	return output.NewStructuredError(
		output.ErrInstallNotInstalled,
		fmt.Sprintf("no install record for %s/%s", c.Type, c.Name),
		"Rollback needs an installed item with update history.",
	)
}

func noRollbackDataError(c installstore.Coord) error {
	return output.NewStructuredError(
		output.ErrInstallConflict,
		fmt.Sprintf("no rollback data for %s/%s", c.Type, c.Name),
		"Rollback is one-step: it becomes available after an update replaces the item.",
	)
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
	fmt.Fprintf(output.Writer, "would roll back %s/%s to %s\n", item.Type, item.Name, shortSHA(prev.SourceSHA))
}

func restoreFromPreviousCopy(libraryPath, copyPath string) error {
	info, err := os.Stat(copyPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return output.NewStructuredError(
				output.ErrInstallConflict,
				fmt.Sprintf("saved previous copy is gone: %s", copyPath),
				"Rollback copy data is local and one-step; restore from backup or re-add the item from its registry.",
			)
		}
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "checking saved previous copy failed", "Check filesystem permissions", err.Error())
	}
	if !info.IsDir() {
		return output.NewStructuredError(output.ErrInstallConflict, fmt.Sprintf("saved previous copy is not a directory: %s", copyPath), "Rollback copy data must be an intact directory tree.")
	}
	if err := os.RemoveAll(libraryPath); err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "removing current library item failed", "Check permissions in ~/.syllago/content/", err.Error())
	}
	if err := copyTree(copyPath, libraryPath); err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "restoring saved previous copy failed", "Check permissions in ~/.syllago/content/ and the saved rollback copy.", err.Error())
	}
	return nil
}

func restoreFromGitPrevious(item catalog.ContentItem, libraryPath, regName, sha string) error {
	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}
	worktreeDir, cleanup, err := registry.WorktreeAt(regName, sha)
	if err != nil {
		return output.NewStructuredErrorDetail(
			output.ErrRegistrySyncFailed,
			fmt.Sprintf("could not materialize rollback source %s", shortSHA(sha)),
			"Run 'syllago registry sync "+regName+"' or restore the registry clone with full history.",
			err.Error(),
		)
	}
	defer cleanup()

	items, err := add.DiscoverFromRegistry(regName, worktreeDir, globalDir)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning rollback registry checkout failed", "Check registry contents at the rollback commit.", err.Error())
	}
	var match *add.DiscoveryItem
	for i := range items {
		if items[i].Type == item.Type && items[i].Name == item.Name {
			match = &items[i]
			break
		}
	}
	if match == nil {
		return output.NewStructuredError(
			output.ErrItemNotFound,
			fmt.Sprintf("item did not exist at %s", shortSHA(sha)),
			"Rollback can only restore items present at the recorded previous registry commit.",
		)
	}

	sourceProvider := item.Provider
	sourceVisibility := ""
	if item.Meta != nil {
		if sourceProvider == "" {
			sourceProvider = item.Meta.SourceProvider
		}
		sourceVisibility = item.Meta.SourceVisibility
	}
	results := add.AddItems([]add.DiscoveryItem{*match}, add.AddOptions{
		Force:            true,
		Provider:         sourceProvider,
		SourceRegistry:   regName,
		SourceSHA:        sha,
		SourceVisibility: sourceVisibility,
	}, globalDir, nil, version)
	if len(results) != 1 {
		return output.NewStructuredError(output.ErrSystemIO, fmt.Sprintf("restoring %s/%s failed", item.Type, item.Name), "The rollback restore did not produce a result.")
	}
	if results[0].Status == add.AddStatusError {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, fmt.Sprintf("restoring %s/%s failed", item.Type, item.Name), "Check registry item contents and library permissions.", results[0].Error.Error())
	}
	if _, err := os.Stat(libraryPath); err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "restored library item is missing", "The registry restore completed but the expected library path was not found.", err.Error())
	}
	return nil
}

func reapplyRollbackPlacements(item catalog.ContentItem, placements []installstore.Placement) []installstore.Placement {
	projectRoot, projectErr := findProjectRoot()
	var reapplied []installstore.Placement
	for _, pl := range placements {
		if !shouldReapplyPlacement(pl.Mechanism) {
			continue
		}
		if projectErr != nil {
			warnRollbackPlacement(pl, projectErr)
			continue
		}
		prov := findProviderBySlug(pl.Provider)
		if prov == nil {
			warnRollbackPlacement(pl, fmt.Errorf("unknown provider"))
			continue
		}
		var err error
		switch pl.Mechanism {
		case installstore.MechanismHookMerge, installstore.MechanismMCPMerge:
			err = reapplyJSONMergePlacement(item, *prov, projectRoot)
		case installstore.MechanismRuleAppend:
			err = reapplyRuleAppendPlacement(item, pl, projectRoot)
		}
		if err != nil {
			warnRollbackPlacement(pl, err)
			continue
		}
		reapplied = append(reapplied, pl)
	}
	return reapplied
}

func shouldReapplyPlacement(m installstore.Mechanism) bool {
	switch m {
	case installstore.MechanismHookMerge, installstore.MechanismMCPMerge, installstore.MechanismRuleAppend:
		return true
	default:
		return false
	}
}

func reapplyJSONMergePlacement(item catalog.ContentItem, prov provider.Provider, projectRoot string) error {
	if _, err := installer.Uninstall(item, prov, projectRoot); err != nil {
		return fmt.Errorf("remove current merge before re-apply: %w", err)
	}
	if _, err := installer.Install(item, prov, projectRoot, installer.MethodSymlink, ""); err != nil {
		return fmt.Errorf("install restored merge: %w", err)
	}
	return nil
}

func reapplyRuleAppendPlacement(item catalog.ContentItem, pl installstore.Placement, projectRoot string) error {
	if item.Type != catalog.Rules {
		return fmt.Errorf("rule_append placement recorded for %s item", item.Type)
	}
	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return fmt.Errorf("cannot determine home directory")
	}
	ruleDir, err := findLibraryRuleDir(filepath.Join(globalDir, string(catalog.Rules)), item.Name)
	if err != nil {
		return err
	}
	loaded, err := rulestore.LoadRule(ruleDir)
	if err != nil {
		return err
	}
	body := loaded.History[loaded.Meta.CurrentVersion]
	if body == nil {
		return fmt.Errorf("no history entry for current_version %s", loaded.Meta.CurrentVersion)
	}
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		return err
	}
	libraryID := loaded.Meta.ID
	for _, r := range inst.RuleAppends {
		if r.Name == item.Name && r.Provider == pl.Provider && r.TargetFile == pl.Path {
			libraryID = r.LibraryID
			break
		}
	}
	library := map[string]*rulestore.Loaded{
		libraryID:      loaded,
		loaded.Meta.ID: loaded,
	}
	return installer.ReplaceRuleAppend(projectRoot, libraryID, pl.Path, body, library)
}

func warnRollbackPlacement(pl installstore.Placement, err error) {
	fmt.Fprintf(output.ErrWriter, "warning: could not re-apply %s placement for %s: %v\n", pl.Mechanism, pl.Provider, err)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyTreeFile(path, target)
	})
}

func copyTreeFile(src, dst string) (err error) {
	if info, err := os.Lstat(dst); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("destination is a symlink: %s", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.CreateTemp(filepath.Dir(dst), ".syllago-*")
	if err != nil {
		return err
	}
	tmpPath := out.Name()
	defer func() {
		if err != nil {
			_ = out.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, dst)
}

func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}
