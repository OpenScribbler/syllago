package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/regdiff"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

type registryStatusJSON struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	State    string `json:"state"`
	Added    int    `json:"added"`
	Modified int    `json:"modified"`
	Removed  int    `json:"removed"`
	Error    string `json:"error,omitempty"`
}

var registryStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show upstream registry changes without syncing",
	Long: `Fetches registry metadata and reports what "registry sync" would pull
without moving git checkouts or persisting MOAT trust/cache state.`,
	Example: `  # Check all registries
  syllago registry status

  # Check a specific registry
  syllago registry status my-rules`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRegistryStatus(cmd.Context(), args)
	},
}

func runRegistryStatus(ctx context.Context, args []string) error {
	lockfileRoot, err := findProjectRoot()
	if err != nil {
		return err
	}
	cfg, err := config.LoadGlobal()
	if err != nil {
		return err
	}

	var regs []config.Registry
	explicit := len(args) == 1
	if explicit {
		reg := findRegistryByName(cfg, args[0])
		if reg == nil {
			telemetry.Enrich("registry_count", 0)
			return output.NewStructuredError(output.ErrRegistryNotFound, fmt.Sprintf("registry %q not found in config", args[0]), "Run 'syllago registry list' to see configured registries")
		}
		regs = append(regs, *reg)
	} else {
		regs = append(regs, cfg.Registries...)
	}

	telemetry.Enrich("registry_count", len(regs))
	if len(regs) == 0 {
		if output.JSON {
			output.Print([]registryStatusJSON{})
		} else {
			fmt.Fprintln(output.Writer, "No registries configured.")
		}
		return nil
	}

	cacheDir, _ := config.GlobalDirPath()
	rows := make([]registryStatusJSON, 0, len(regs))
	for i := range regs {
		reg := &regs[i]
		row, err := registryStatusOne(ctx, reg, lockfileRoot, cacheDir)
		if err != nil {
			if explicit {
				return err
			}
			row = registryStatusJSON{
				Name:  reg.Name,
				Kind:  registryStatusKind(reg),
				State: "error",
				Error: err.Error(),
			}
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "warning: %s: %s\n", reg.Name, err)
				continue
			}
		}
		rows = append(rows, row)
	}

	if output.JSON {
		output.Print(rows)
	}
	return nil
}

func registryStatusOne(ctx context.Context, reg *config.Registry, lockfileRoot, cacheDir string) (registryStatusJSON, error) {
	if reg.IsMOAT() {
		return registryStatusMOAT(ctx, reg, lockfileRoot, cacheDir)
	}
	return registryStatusGit(reg)
}

func registryStatusMOAT(ctx context.Context, reg *config.Registry, lockfileRoot, cacheDir string) (registryStatusJSON, error) {
	row := registryStatusJSON{Name: reg.Name, Kind: "moat"}
	outcome, err := registryops.StatusOne(ctx, reg.Name, registryops.SyncOpts{
		LockfileRoot: lockfileRoot,
		CacheDir:     cacheDir,
		Now:          time.Now(),
	})
	if err != nil {
		return row, classifyMOATStatusError(reg.Name, err)
	}

	switch {
	case outcome.UpToDate:
		row.State = "up_to_date"
		registryStatusPrintLine("%s (moat): up to date\n", reg.Name)
	case outcome.ProfileChanged:
		row.State = "profile_changed"
		registryStatusPrintLine("%s (moat): signing profile changed upstream — run 'syllago registry sync %s'\n", reg.Name, reg.Name)
	case outcome.NotSynced:
		row.State = "not_synced"
		registryStatusPrintLine("%s (moat): not synced yet — run 'syllago registry sync %s'\n", reg.Name, reg.Name)
	case outcome.Diff == nil:
		row.State = "error"
		row.Error = "could not compare"
		registryStatusPrintLine("%s (moat): could not compare — run 'syllago registry sync %s'\n", reg.Name, reg.Name)
	case registryStatusHasChanges(outcome.Diff):
		registryStatusSetChangeCounts(&row, outcome.Diff)
		registryStatusPrintDiff(reg.Name, "moat", outcome.Diff)
	default:
		row.State = "up_to_date"
		registryStatusPrintLine("%s (moat): up to date\n", reg.Name)
	}

	return row, nil
}

func registryStatusGit(reg *config.Registry) (registryStatusJSON, error) {
	row := registryStatusJSON{Name: reg.Name, Kind: "git"}
	outcome, err := registry.Status(reg.Name)
	if err != nil {
		return row, output.NewStructuredErrorDetail(output.ErrRegistrySyncFailed, fmt.Sprintf("status failed for %q", reg.Name), "Check network connectivity and git credentials", err.Error())
	}
	if outcome.Head == outcome.RemoteHead {
		row.State = "up_to_date"
		registryStatusPrintLine("%s (git): up to date\n", reg.Name)
		return row, nil
	}

	d := registryops.GitSyncDiff(reg.Name, outcome.Head, outcome.RemoteHead)
	if d == nil {
		row.State = "error"
		row.Error = "could not compare"
		registryStatusPrintLine("%s (git): could not compare — run 'syllago registry sync %s'\n", reg.Name, reg.Name)
		return row, nil
	}
	cloneDir, err := registry.CloneDir(reg.Name)
	if err != nil {
		row.State = "error"
		row.Error = "could not compare"
		registryStatusPrintLine("%s (git): could not compare — run 'syllago registry sync %s'\n", reg.Name, reg.Name)
		return row, nil
	}
	refineRegistryStatusGitKinds(cloneDir, outcome.Head, outcome.RemoteHead, d)
	if registryStatusHasChanges(d) {
		registryStatusSetChangeCounts(&row, d)
		registryStatusPrintDiff(reg.Name, "git", d)
		return row, nil
	}

	row.State = "up_to_date"
	registryStatusPrintLine("%s (git): up to date\n", reg.Name)
	return row, nil
}

func registryStatusPrintLine(format string, args ...any) {
	if output.JSON {
		return
	}
	fmt.Fprintf(output.Writer, format, args...)
}

func registryStatusPrintDiff(name, kind string, d *regdiff.Diff) {
	if output.JSON {
		return
	}
	fmt.Fprintf(output.Writer, "%s (%s): %d upstream change(s)\n", name, kind, len(d.Changes))
	printRegistryDiffLines(output.Writer, d)
}

func registryStatusSetChangeCounts(row *registryStatusJSON, d *regdiff.Diff) {
	row.State = "changes"
	for _, change := range d.Changes {
		switch change.Kind {
		case regdiff.KindAdded:
			row.Added++
		case regdiff.KindRemoved:
			row.Removed++
		default:
			row.Modified++
		}
	}
}

func registryStatusHasChanges(d *regdiff.Diff) bool {
	return d != nil && (len(d.Changes) > 0 || len(d.OtherPaths) > 0)
}

// refineRegistryStatusGitKinds corrects added/removed classification for the
// status flow. GitDiff's kind logic assumes the working tree sits at newRef
// (true after sync's pull): items scanned from the checkout decide which
// accumulator a path lands in, and only one side runs each existence check.
// During status the checkout is still at oldRef, so upstream adds and removes
// would both surface as Modified; re-checking path existence at both refs
// restores the right kinds without moving the checkout.
func refineRegistryStatusGitKinds(repoDir, oldRef, newRef string, d *regdiff.Diff) {
	if d == nil {
		return
	}
	for i := range d.Changes {
		paths := d.Changes[i].Paths
		if len(paths) == 0 {
			continue
		}
		oldAny, oldAll := registryStatusGitPathSetExists(repoDir, oldRef, paths)
		newAny, newAll := registryStatusGitPathSetExists(repoDir, newRef, paths)
		switch {
		case !oldAny && newAll:
			d.Changes[i].Kind = regdiff.KindAdded
		case oldAll && !newAny:
			d.Changes[i].Kind = regdiff.KindRemoved
		}
	}
}

func registryStatusGitPathSetExists(repoDir, ref string, paths []string) (anyExist, allExist bool) {
	allExist = true
	for _, path := range paths {
		exists := registryStatusGitPathExists(repoDir, ref, path)
		anyExist = anyExist || exists
		allExist = allExist && exists
	}
	return anyExist, allExist
}

func registryStatusGitPathExists(repoDir, ref, path string) bool {
	cmd := exec.Command("git", "-C", repoDir, "cat-file", "-e", ref+":"+path)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	return cmd.Run() == nil
}

func registryStatusKind(reg *config.Registry) string {
	if reg.IsMOAT() {
		return "moat"
	}
	return "git"
}

func classifyMOATStatusError(name string, err error) error {
	if status, ok := registryops.IsTrustedRootStale(err); ok {
		return output.NewStructuredErrorDetail(
			output.ErrMoatTrustedRootStale,
			fmt.Sprintf("bundled trusted root unusable while checking registry %q", name),
			"Run `syllago update` to refresh the bundled Sigstore trusted root.",
			status.String(),
		)
	}
	var ve *moat.VerifyError
	if errors.As(err, &ve) {
		return classifyVerifyError(name, err)
	}
	return output.NewStructuredErrorDetail(
		output.ErrMoatInvalid,
		fmt.Sprintf("status failed for registry %q", name),
		"Run `syllago registry sync` after checking network connectivity and the registry's manifest URL.",
		err.Error(),
	)
}
