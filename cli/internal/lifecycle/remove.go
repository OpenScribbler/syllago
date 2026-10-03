package lifecycle

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
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
	Present []Target                // providers that hold the item at their default location
	Appends []Target                // monolithic rule files the item is appended to, one per File
	Unknown []Failure               // checks that could not tell; Remove stops on any
	Links   []installer.ScannedLink // links into the item that no Present target accounts for

	dests map[string]string // install path of each Present target, by provider slug
}

// Remove uninstalls the item from every provider that holds it, takes it
// out of every rule file it is appended to, removes every provider link
// into it, then deletes it from the Library and forgets its install record.
//
// Without Decisions.RemoveConfirmed it changes nothing and returns
// *DecisionRequired carrying the RemovePlan. With it, Remove stops before
// any change when a check could not tell where the item is installed or the
// install records cannot be read. Otherwise it attempts every target and
// link; if any fails, or the record names an install Remove did not reach
// and that is still in place, the Library item and the record stay, and
// the record loses only the placements that were removed. A record that
// cannot be forgotten after the Library item is gone appears only in
// Outcome.Failed.
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
	rec := m.loadRemovalRecord(req.Item)
	if rec.err != nil {
		plan.Unknown = append(plan.Unknown, Failure{Stage: StagePresence, Err: recordErr(rec.err)})
	}
	if len(plan.Unknown) > 0 {
		out.Failed = append(out.Failed, plan.Unknown...)
		out.Unattempted = append(plan.Present, plan.Appends...)
		var errs []error
		for _, f := range plan.Unknown {
			errs = append(errs, f.Err)
		}
		return out, fmt.Errorf("cannot tell where %s is installed, so nothing was removed: %w", req.Item.Name, errors.Join(errs...))
	}

	var errs []error
	unplace := func(ureq UninstallRequest, t Target) (installer.Placement, bool) {
		pl, err := m.placer.unplace(ureq, t)
		if err != nil {
			out.Failed = append(out.Failed, Failure{Target: t, Stage: StagePlace, Err: err})
			errs = append(errs, err)
			return pl, false
		}
		out.Changed = true
		out.Completed = append(out.Completed, Step{Target: t, Placement: pl})
		return pl, true
	}
	// Providers that share an install directory hold one install between
	// them, and removing it for the first removes it for the rest.
	removed := make(map[string]installer.Placement)
	for _, t := range plan.Present {
		dest := plan.dests[t.Provider.Slug]
		if pl, ok := removed[dest]; ok {
			out.Completed = append(out.Completed, Step{Target: t, Placement: pl})
			continue
		}
		if pl, ok := unplace(UninstallRequest{Item: req.Item, ProjectRoot: req.ProjectRoot}, t); ok && dest != "" {
			removed[dest] = pl
		}
	}
	for _, t := range plan.Appends {
		unplace(UninstallRequest{Item: req.Item, ProjectRoot: req.ProjectRoot, Method: installer.MethodAppend}, t)
	}
	for _, link := range plan.Links {
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

	rec.drop(out.Completed, m.now())
	if len(errs) == 0 {
		for _, f := range rec.unreached(req.Item, req.Providers) {
			out.Failed = append(out.Failed, f)
			errs = append(errs, f.Err)
		}
	}
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

// removePlan checks every provider for the item, reads the rule files it
// is appended to, and sweeps for links into it. It changes nothing.
func (m *Module) removePlan(req RemoveRequest) RemovePlan {
	plan := RemovePlan{dests: make(map[string]string)}
	home, _ := os.UserHomeDir()
	reached := make(map[string]bool)
	for _, p := range req.Providers {
		t := Target{Provider: p}
		ok, err := m.placer.present(req.Item, req.ProjectRoot, t)
		var dest string
		if ok && err == nil {
			dest, err = installer.InstallPath(req.Item, p, home)
		}
		if err != nil {
			plan.Unknown = append(plan.Unknown, Failure{Target: t, Stage: StagePresence, Err: fmt.Errorf("checking %s: %w", p.Name, err)})
			continue
		}
		if ok {
			plan.Present = append(plan.Present, t)
			if dest != "" {
				plan.dests[p.Slug] = dest
				reached[dest] = true
			}
		}
	}
	appends, err := m.placer.appended(req.Item, req.ProjectRoot)
	if err != nil {
		plan.Unknown = append(plan.Unknown, Failure{Stage: StagePresence, Err: fmt.Errorf("reading rule appends: %w", err)})
	}
	for _, t := range appends {
		named := targetFor(req.Providers, t.Provider.Slug)
		named.File = t.File
		plan.Appends = append(plan.Appends, named)
	}
	links, scanErrs := installer.ScanProviderLinksChecked(req.Providers, home, []string{req.Item.Path})
	for _, e := range scanErrs {
		plan.Unknown = append(plan.Unknown, Failure{Target: targetFor(req.Providers, e.Provider), Stage: StagePresence, Err: e})
	}
	for _, link := range links {
		if reached[link.Path] {
			continue
		}
		plan.Links = append(plan.Links, link)
	}
	return plan
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

// loadRemovalRecord loads the store that holds the item's install record.
func (m *Module) loadRemovalRecord(item catalog.ContentItem) *removalRecord {
	r := &removalRecord{coord: recordCoord(item)}
	path, err := installstore.DefaultPath()
	if err == nil {
		r.store, err = installstore.Load(path)
	}
	r.err = err
	return r
}

// drop removes the placements of steps from the item's record.
func (r *removalRecord) drop(steps []Step, now time.Time) {
	if r.store == nil {
		return
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
		rec.UpdatedAt = now
	}
}

// unreached returns a failure for each install the record still lists that
// is still in place: one Remove did not reach, such as a copy under a custom
// base directory or an MCP server merged into another project's config.
// Deleting the Library item would orphan it. A merged hook is not checked,
// because its placement names the event it joined rather than the hook.
func (r *removalRecord) unreached(item catalog.ContentItem, providers []provider.Provider) []Failure {
	if r.store == nil {
		return nil
	}
	rec := r.store.Find(r.coord)
	if rec == nil {
		return nil
	}
	var out []Failure
	var rule *rulestore.Loaded
	var ruleErr error
	for _, p := range rec.Placements {
		var there bool
		var err error
		switch p.Mechanism {
		case installstore.MechanismSymlink, installstore.MechanismCopy:
			_, err = os.Lstat(p.Path)
			there = err == nil
			if errors.Is(err, fs.ErrNotExist) {
				err = nil
			}
		case installstore.MechanismMCPMerge:
			there, err = installer.MCPKeyIn(p.Path, p.Key)
		case installstore.MechanismRuleAppend:
			if rule == nil && ruleErr == nil {
				rule, ruleErr = rulestore.LoadRule(item.Path)
			}
			err = ruleErr
			if err == nil {
				there, err = installer.RuleAppendedIn(p.Path, rule)
			}
		default:
			continue
		}
		if err == nil && !there {
			continue
		}
		if err == nil {
			err = fmt.Errorf("still installed at %s, which Remove does not reach; uninstall it there, then remove again", p.Path)
		} else {
			err = fmt.Errorf("checking %s: %w", p.Path, err)
		}
		out = append(out, Failure{Target: targetFor(providers, p.Provider), Stage: StagePlace, Err: err})
	}
	return out
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
