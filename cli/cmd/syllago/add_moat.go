package main

// MOAT registry-sourced add: `syllago add [type[/name]] --from <registry>`.
//
// moatinstall.Operation does the work, as for `syllago install`: it syncs
// the registry, checks each item against the trust gate, and fetches,
// verifies and stages it into the Library. An add passes no install
// targets, so nothing reaches a provider.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

// runAddFromMOATRegistry lists what a MOAT registry offers, or adds the
// items args and addAll select to the Library. A G-18 non-interactive
// failure calls moatSyncExit(code) and returns nil, as install does.
func runAddFromMOATRegistry(ctx context.Context, projectRoot string, reg *config.Registry, args []string, fromSlug string, addAll, dryRun, force bool, globalDir, trustedRootOverride string) error {
	if trustedRootOverride != "" {
		return output.NewStructuredError(output.ErrInputConflict,
			"--trusted-root is not supported when adding from a MOAT registry",
			"The registry sync verifies against the bundled Sigstore trusted root; run `syllago update` to refresh it")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	errW := output.ErrWriter
	now := moatInstallNow()
	op := moatinstall.Operation{MinTier: moatInstallMinTier, Now: func() time.Time { return now }}

	// A request for no items syncs the registry and reads its manifest.
	req := moatinstall.Request{Registry: reg.Name, ProjectRoot: projectRoot, DryRun: true, Session: moat.NewSession()}
	res, ok, err := runRegistryAdd(ctx, errW, &op, &req, false)
	if !ok {
		return err
	}
	items := moatDiscoveryItems(res.Manifest, reg.Name, globalDir)

	if len(args) == 0 && !addAll {
		return printDiscoveryText(reg.Name, reg.Name, items)
	}
	var typeStr, nameFilter string
	if len(args) > 0 {
		typeStr, nameFilter, _ = strings.Cut(args[0], "/")
	}
	items, err = filterDiscoveryItems(items, typeStr, nameFilter, "registry "+reg.Name)
	if err != nil {
		return err
	}

	// The Library already holds this registry's copy of an up-to-date item,
	// an outdated one waits for --force, and a copy from another source
	// cannot be replaced, so none of them is fetched.
	var results []add.AddResult
	var failed error
	outdated := map[moatinstall.Item]bool{}
	req.Items = nil
	for _, item := range items {
		ref := moatinstall.Item{Name: item.Name, Type: item.Type}
		switch {
		case item.Status == add.StatusInLibrary:
			results = append(results, add.AddResult{Name: item.Name, Type: item.Type, Status: add.AddStatusUpToDate})
		case item.Status == add.StatusOutdated && !fromRegistry(globalDir, item, reg.Name):
			err := fmt.Errorf("the Library holds %s/%s from another source; remove it to add this registry's copy", item.Type, item.Name)
			results = append(results, add.AddResult{Name: item.Name, Type: item.Type, Status: add.AddStatusError, Error: err})
			if failed == nil {
				failed = err
			}
		case item.Status == add.StatusOutdated && !force:
			results = append(results, add.AddResult{Name: item.Name, Type: item.Type, Status: add.AddStatusSkipped})
		default:
			outdated[ref] = item.Status == add.StatusOutdated
			req.Items = append(req.Items, ref)
		}
	}

	if len(req.Items) > 0 {
		req.DryRun = dryRun
		res, ok, err = runRegistryAdd(ctx, errW, &op, &req, len(results) > 0)
		if !ok {
			return err
		}
		for _, ir := range res.Items {
			r := add.AddResult{Name: ir.Name, Status: add.AddStatusAdded}
			if ir.Entry != nil {
				r.Type, _ = moat.FromMOATType(ir.Entry.Type)
			}
			switch {
			case errors.Is(ir.Err, moatinstall.ErrDeclined):
				// A pinned item, already reported.
				continue
			case ir.Err != nil:
				r.Status, r.Error = add.AddStatusError, ir.Err
				if failed == nil {
					failed = ir.Err
				}
			case outdated[moatinstall.Item{Name: ir.Name, Type: r.Type}]:
				r.Status = add.AddStatusUpdated
			}
			results = append(results, r)
		}
		printLifecycleWarnings(errW, res.Stage)
	}

	telemetry.Enrich("from", fromSlug)
	telemetry.Enrich("content_type", typeStr)
	telemetry.Enrich("content_count", len(results))
	telemetry.Enrich("dry_run", dryRun)
	if err := printAddResults(results, dryRun, reg.Name); err != nil {
		return err
	}
	// An item the registry could not vouch for fails the command, so a
	// script never reads a refused verification as success.
	return failed
}

// runRegistryAdd runs op.Install, answering each decision it asks for.
// othersReported says the add has results for items it did not request, so
// a request whose every item is pinned still leaves something to report.
// It reports ok=false when the add stops, with the error to return; a
// headless refusal has already exited.
func runRegistryAdd(ctx context.Context, errW io.Writer, op *moatinstall.Operation, req *moatinstall.Request, othersReported bool) (moatinstall.Result, bool, error) {
	for {
		res, err := op.Install(ctx, *req)
		var decision *lifecycle.DecisionRequired
		if errors.As(err, &decision) {
			retry, derr := decideRegistryAdd(errW, req, decision, othersReported)
			if retry {
				continue
			}
			return res, false, derr
		}
		if err != nil {
			return res, false, registryInstallError(errW, req.Registry, err)
		}
		return res, true, nil
	}
}

// decideRegistryAdd answers a decision as install does, except that a
// pinned Library item is left as it is while the other items are added.
func decideRegistryAdd(errW io.Writer, req *moatinstall.Request, decision *lifecycle.DecisionRequired, othersReported bool) (bool, error) {
	if decision.Kind != lifecycle.OverwritePinned {
		return decideRegistryInstall(errW, req, decision)
	}
	pinned, _ := decision.Context.([]lifecycle.PinnedDestination)
	if req.Decisions.Overwrite == nil {
		req.Decisions.Overwrite = make(map[string]bool, len(pinned))
	}
	for _, p := range pinned {
		req.Decisions.Overwrite[p.Path] = false
		fmt.Fprintf(errW, "  pinned %s/%s; unpin to update: syllago unpin %s\n", p.Type, p.Name, p.Name)
	}
	if len(pinned) == len(req.Items) && !othersReported {
		return false, output.NewStructuredError(
			output.ErrInstallConflict,
			"all requested items are pinned",
			"Run 'syllago unpin <name>' for each item you want to update",
		)
	}
	return true, nil
}

// fromRegistry reports whether the Library's copy of item came from the
// registry regName.
func fromRegistry(globalDir string, item add.DiscoveryItem, regName string) bool {
	meta, _ := metadata.Load(filepath.Join(globalDir, string(item.Type), item.Name))
	return meta != nil && meta.SourceRegistry == regName
}

// moatDiscoveryItems lists the manifest's items with their Library status:
// in the Library when this registry's copy holds the manifest's hash,
// outdated when it holds another, and new otherwise. An item the Library
// holds from another source is outdated too, and the add refuses it.
func moatDiscoveryItems(manifest *moat.Manifest, regName, globalDir string) []add.DiscoveryItem {
	if manifest == nil {
		return nil
	}
	var items []add.DiscoveryItem
	for _, entry := range manifest.Content {
		ct, ok := moat.FromMOATType(entry.Type)
		if !ok {
			continue
		}
		dir := filepath.Join(globalDir, string(ct), entry.Name)
		status := add.StatusNew
		if _, err := os.Stat(dir); err == nil {
			status = add.StatusOutdated
			if meta, _ := metadata.Load(dir); meta != nil && meta.SourceRegistry == regName && meta.SourceHash == entry.ContentHash {
				status = add.StatusInLibrary
			}
		}
		items = append(items, add.DiscoveryItem{Name: entry.Name, Type: ct, Status: status, DisplayName: entry.DisplayName})
	}
	return items
}
