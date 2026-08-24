package rollback

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
)

// Plan describes a resolved one-step rollback.
type Plan struct {
	Item        catalog.ContentItem
	Coord       installstore.Coord
	StorePath   string
	LibraryPath string
	Prev        installstore.PreviousVersion
	Placements  []installstore.Placement
	FromCopy    bool
}

// Options carries cmd-layer context into ReapplyPlacements.
type Options struct {
	ProjectRoot    string
	ProjectRootErr error
}

// PlanFor resolves the install record and previous version data for item.
func PlanFor(item catalog.ContentItem) (*Plan, error) {
	if item.Meta == nil || item.Meta.SourceRegistry == "" {
		return nil, output.NewStructuredError(
			output.ErrInputInvalid,
			fmt.Sprintf("%s/%s is not registry-sourced", item.Type, item.Name),
			"Rollback needs an item installed from a registry update.",
		)
	}

	storePath, rec, coord, err := loadRollbackRecord(item)
	if err != nil {
		return nil, err
	}
	if rec.Previous == nil || rec.Previous.SourceSHA == "" && rec.Previous.CopyPath == "" {
		return nil, noRollbackDataError(coord)
	}

	prev := *rec.Previous
	return &Plan{
		Item:        item,
		Coord:       coord,
		StorePath:   storePath,
		LibraryPath: rec.LibraryPath,
		Prev:        prev,
		Placements:  append([]installstore.Placement(nil), rec.Placements...),
		FromCopy:    prev.CopyPath != "",
	}, nil
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

// Restore restores plan's previous content and records the rollback.
func Restore(plan *Plan, version string) error {
	if plan.FromCopy {
		if err := restoreFromPreviousCopy(plan.LibraryPath, plan.Prev.CopyPath); err != nil {
			return err
		}
	} else {
		if err := restoreFromGitPrevious(plan.Item, plan.LibraryPath, plan.Coord.Registry, plan.Prev.SourceSHA, version); err != nil {
			return err
		}
	}

	if err := installstore.RecordRollback(plan.StorePath, plan.Coord, plan.LibraryPath, time.Now()); err != nil {
		return output.NewStructuredErrorDetail(
			output.ErrSystemIO,
			"content was restored but install record rollback failed",
			"Inspect the library item and install record before running another update.",
			err.Error(),
		)
	}
	return nil
}

// ReapplyPlacements re-applies merge and append placements for a restored item.
func ReapplyPlacements(item catalog.ContentItem, placements []installstore.Placement, opts Options) (reapplied []installstore.Placement, warnings []string) {
	for _, pl := range placements {
		if !shouldReapplyPlacement(pl.Mechanism) {
			continue
		}
		if opts.ProjectRootErr != nil {
			warnings = append(warnings, rollbackPlacementWarning(pl, opts.ProjectRootErr))
			continue
		}
		prov := findProviderBySlug(pl.Provider)
		if prov == nil {
			warnings = append(warnings, rollbackPlacementWarning(pl, fmt.Errorf("unknown provider")))
			continue
		}
		var err error
		switch pl.Mechanism {
		case installstore.MechanismHookMerge, installstore.MechanismMCPMerge:
			err = reapplyJSONMergePlacement(item, *prov, opts.ProjectRoot)
		case installstore.MechanismRuleAppend:
			err = reapplyRuleAppendPlacement(item, pl, opts.ProjectRoot)
		}
		if err != nil {
			warnings = append(warnings, rollbackPlacementWarning(pl, err))
			continue
		}
		reapplied = append(reapplied, pl)
	}
	return reapplied, warnings
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
	ruleDir, err := rulestore.FindRuleDir(filepath.Join(globalDir, string(catalog.Rules)), item.Name)
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

func rollbackPlacementWarning(pl installstore.Placement, err error) string {
	return fmt.Sprintf("could not re-apply %s placement for %s: %v", pl.Mechanism, pl.Provider, err)
}

func findProviderBySlug(slug string) *provider.Provider {
	for i := range provider.AllProviders {
		if provider.AllProviders[i].Slug == slug {
			return &provider.AllProviders[i]
		}
	}
	return nil
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

func restoreFromGitPrevious(item catalog.ContentItem, libraryPath, regName, sha, version string) error {
	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}
	worktreeDir, cleanup, err := registry.WorktreeAt(regName, sha)
	if err != nil {
		return output.NewStructuredErrorDetail(
			output.ErrRegistrySyncFailed,
			fmt.Sprintf("could not materialize rollback source %s", ShortSHA(sha)),
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
			fmt.Sprintf("item did not exist at %s", ShortSHA(sha)),
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

// ShortSHA returns sha shortened to the CLI display length.
func ShortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}
