package lifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// RemoveRequest carries everything one Remove call needs.
type RemoveRequest struct {
	Item        catalog.ContentItem
	ProjectRoot string
	// Providers are every provider Remove checks. Remove always means all
	// of them; it checks each at its default install location.
	Providers []provider.Provider
	Decisions Decisions
}

// RemovePlan is what a Remove would do, returned as the Context of its
// RemoveConfirm decision.
type RemovePlan struct {
	Present []Target                // providers that hold the item
	Unknown []Failure               // providers whose check could not tell; Remove stops on any
	Links   []installer.ScannedLink // links into the item that no Present target accounts for
}

// Remove uninstalls the item from every provider that holds it, removes
// every provider link into it, then deletes it from the Library and forgets
// its install record.
//
// Without Decisions.RemoveConfirmed it changes nothing and returns
// *DecisionRequired carrying the RemovePlan. With it, Remove stops before
// any change when a provider's check could not tell whether it holds the
// item. Otherwise it attempts every target and link; if any fails, the
// Library item and the record stay, and the record loses only the
// placements that were removed. A record that cannot be forgotten after the
// Library item is gone appears only in Outcome.Failed.
func (m *Module) Remove(req RemoveRequest) (Outcome, error) {
	var out Outcome
	if !req.Decisions.RemoveConfirmed {
		return out, &DecisionRequired{Kind: RemoveConfirm, Context: m.removePlan(req)}
	}
	release, err := m.lock()
	if err != nil {
		out.Unattempted = providerTargets(req.Providers)
		return out, err
	}
	defer release()

	plan := m.removePlan(req)
	if len(plan.Unknown) > 0 {
		out.Failed = append(out.Failed, plan.Unknown...)
		out.Unattempted = plan.Present
		var errs []error
		for _, f := range plan.Unknown {
			errs = append(errs, f.Err)
		}
		return out, fmt.Errorf("cannot tell where %s is installed, so nothing was removed: %w", req.Item.Name, errors.Join(errs...))
	}

	var errs []error
	ureq := UninstallRequest{Item: req.Item, ProjectRoot: req.ProjectRoot}
	for _, t := range plan.Present {
		pl, err := m.placer.unplace(ureq, t)
		if err != nil {
			out.Failed = append(out.Failed, Failure{Target: t, Stage: StagePlace, Err: err})
			errs = append(errs, err)
			continue
		}
		out.Changed = true
		out.Completed = append(out.Completed, Step{Target: t, Placement: pl})
	}
	home, _ := os.UserHomeDir()
	for _, link := range installer.ScanProviderLinks(req.Providers, home, []string{req.Item.Path}) {
		t := targetFor(req.Providers, link.Provider)
		if err := os.Remove(link.Path); err != nil {
			err = fmt.Errorf("removing provider link %s: %w", link.Path, err)
			out.Failed = append(out.Failed, Failure{Target: t, Stage: StagePlace, Err: err})
			errs = append(errs, err)
			continue
		}
		out.Changed = true
		out.Completed = append(out.Completed, Step{Target: t, Placement: installer.Placement{Mechanism: installer.MechanismSymlink, Path: link.Path}})
	}

	rec := m.removeRecord(req.Item, out.Completed)
	if len(errs) > 0 {
		out.Failed = append(out.Failed, rec.save()...)
		return out, errors.Join(errs...)
	}
	if err := catalog.RemoveLibraryItem(req.Item.Path); err != nil {
		out.Failed = append(out.Failed, rec.save()...)
		return out, fmt.Errorf("removing %s from the library: %w", req.Item.Name, err)
	}
	rec.forget()
	out.Failed = append(out.Failed, rec.save()...)
	return out, nil
}

// removePlan checks every provider for the item and sweeps for links into
// it. It changes nothing.
func (m *Module) removePlan(req RemoveRequest) RemovePlan {
	var plan RemovePlan
	present := make(map[string]bool)
	for _, p := range req.Providers {
		t := Target{Provider: p}
		ok, err := m.placer.present(req.Item, req.ProjectRoot, t)
		if err != nil {
			plan.Unknown = append(plan.Unknown, Failure{Target: t, Stage: StagePresence, Err: fmt.Errorf("checking %s: %w", p.Name, err)})
			continue
		}
		if ok {
			plan.Present = append(plan.Present, t)
			present[p.Slug] = true
		}
	}
	home, _ := os.UserHomeDir()
	leaf := installLeaf(req.Item)
	for _, link := range installer.ScanProviderLinks(req.Providers, home, []string{req.Item.Path}) {
		if present[link.Provider] && link.ContentType == req.Item.Type && filepath.Base(link.Path) == leaf {
			continue
		}
		plan.Links = append(plan.Links, link)
	}
	return plan
}

// installLeaf is the file or directory name an install of item gets inside
// a provider's install directory.
func installLeaf(item catalog.ContentItem) string {
	if item.Type == catalog.Agents {
		return item.Name + ".md"
	}
	if item.Type.IsUniversal() {
		return item.Name
	}
	return filepath.Base(item.Path)
}

func providerTargets(providers []provider.Provider) []Target {
	var out []Target
	for _, p := range providers {
		out = append(out, Target{Provider: p})
	}
	return out
}

// targetFor returns the target for the provider with slug, or a target
// naming only the slug when no provider in providers has it.
func targetFor(providers []provider.Provider, slug string) Target {
	for _, p := range providers {
		if p.Slug == slug {
			return Target{Provider: p}
		}
	}
	return Target{Provider: provider.Provider{Name: slug, Slug: slug}}
}

// removalRecord is the item's install record, edited in memory so a Remove
// writes it once. Removing placements one at a time through the store
// would prune the record with its last placement, losing its pin and
// provenance while the Library item may yet stay.
type removalRecord struct {
	store   *installstore.Store
	coord   installstore.Coord
	err     error
	changed bool
}

// removeRecord loads the store and drops the placements of steps from the
// item's record.
func (m *Module) removeRecord(item catalog.ContentItem, steps []Step) *removalRecord {
	r := &removalRecord{coord: recordCoord(item)}
	path, err := installstore.DefaultPath()
	if err == nil {
		r.store, err = installstore.Load(path)
	}
	if err != nil {
		r.err = err
		return r
	}
	for _, s := range steps {
		p := recordPlacement(s.Target.Provider.Slug, s.Placement)
		keys := p.Keys
		if len(keys) == 0 {
			keys = []string{""}
		}
		for _, key := range keys {
			if r.store.RemovePlacement(r.coord, p.Provider, p.Mechanism, p.Path, key) {
				r.changed = true
			}
		}
	}
	if rec := r.store.Find(r.coord); rec != nil && r.changed {
		rec.UpdatedAt = m.now()
	}
	return r
}

func (r *removalRecord) forget() {
	if r.store != nil && r.store.Remove(r.coord) {
		r.changed = true
	}
}

// save writes the record when it changed, and returns the failure to
// report, if any.
func (r *removalRecord) save() []Failure {
	err := r.err
	if err == nil && r.changed {
		err = r.store.Save()
	}
	if err != nil {
		return []Failure{{Stage: StageRecord, Err: recordErr(err)}}
	}
	return nil
}
