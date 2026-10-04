package moatinstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/regdiff"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
)

// Operation puts MOAT registry items into the Library and, given targets,
// installs them. The zero value is ready to use; tests replace its seams.
type Operation struct {
	// Sync syncs one registry. Nil means registryops.SyncOne.
	Sync func(ctx context.Context, name string, opts registryops.SyncOpts) (registryops.SyncOutcome, error)
	// Clone clones a source repository. Nil means moat.CloneRepoFn.
	Clone moat.CloneRepoFunc
	// Lifecycle stages and installs. Nil means lifecycle.New().
	Lifecycle *lifecycle.Module
	// MinTier is the lowest trust tier the gate lets through.
	MinTier moat.TrustTier
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
}

// Request names the items to put into the Library and where to install
// them.
type Request struct {
	Registry string
	Items    []Item
	// ProjectRoot holds the MOAT lockfile, and project installs land there.
	ProjectRoot string
	// Targets are the providers to install to. Empty means fetch, verify
	// and stage into the Library only.
	Targets []lifecycle.Target
	Method  installer.InstallMethod
	Scan    installer.ScanOptions
	Frozen  bool // pin each item after it installs
	// DryRun stops after the gate: nothing is fetched or written beyond
	// what the sync itself saves.
	DryRun    bool
	Decisions Decisions
	// Session remembers the warnings the user confirmed. Nil means a new
	// one for this call.
	Session *moat.Session
}

// Decisions carries the choices the user already made, so Install goes
// ahead where it would otherwise return *lifecycle.DecisionRequired.
type Decisions struct {
	lifecycle.Decisions // Overwrite answers OverwritePinned
	// AcceptTOFU trusts the signing profile a registry presents on its
	// first sync.
	AcceptTOFU bool
	// PublisherWarn answers PublisherWarn by content hash, so an answer
	// covers only the content the user saw: true installs the revoked
	// item, false declines it.
	PublisherWarn map[string]bool
	// PrivateSource answers PrivateSource by content hash.
	PrivateSource map[string]bool
}

// GatePrompt is one item a PublisherWarn or PrivateSource decision asks
// about. A *lifecycle.DecisionRequired of either kind carries a
// []GatePrompt; one of kind TrustOnFirstUse carries the
// config.SigningProfile the registry presented.
type GatePrompt struct {
	Entry *moat.ContentEntry
	Gate  installer.GateBlock
}

// Result reports what Install did, as far as it got.
type Result struct {
	Sync      registryops.SyncOutcome
	Staleness moat.StalenessStatus // of the manifest the gate read
	// Manifest is the manifest the gate read, so a caller can list what
	// the registry offers. Nil when the sync stopped the call.
	Manifest *moat.Manifest
	// Stage is the outcome of staging every fetched item into the Library.
	Stage lifecycle.Outcome
	Items []ItemResult // in Request.Items order
}

// Item names one item in the registry's manifest. A manifest may list one
// name under several types, so a caller that knows the type sets it; an
// empty Type takes the first item with the name.
type Item struct {
	Name string
	Type catalog.ContentType
}

// ItemResult reports one requested item.
type ItemResult struct {
	Name  string
	Entry *moat.ContentEntry // nil when the manifest does not list Name
	// Gate is the first gate decision for the item, before any confirmed
	// warning was applied.
	Gate    installer.GateBlock
	Library catalog.ContentItem // the staged item; zero until it is staged
	// Unsupported are the targets that cannot take the item's type. The
	// item installs to the others.
	Unsupported []lifecycle.Target
	Install     lifecycle.Outcome
	// Err is why the item stopped: refused at the gate, declined, not
	// fetched or verified, not staged, or not installed to some target.
	Err error
}

var (
	// ErrDeclined is an ItemResult.Err for an item whose warning or pinned
	// Library copy the user declined.
	ErrDeclined = errors.New("declined by the user")
	// ErrProfileChanged reports that the registry's signing profile no
	// longer matches the one pinned for it. Nothing was saved.
	ErrProfileChanged = errors.New("registry signing profile changed")
	// ErrManifestExpired reports that the registry's manifest is past its
	// expiry.
	ErrManifestExpired = errors.New("registry manifest expired")
)

// SyncError wraps a failed sync. Unwrap reaches a *moat.VerifyError when
// the manifest failed verification.
type SyncError struct {
	Registry string
	Err      error
}

func (e *SyncError) Error() string {
	return fmt.Sprintf("sync of registry %q failed: %v", e.Registry, e.Err)
}

func (e *SyncError) Unwrap() error { return e.Err }

// fetched is an item verified and copied into the source cache.
type fetched struct {
	idx         int
	cacheDir    string
	rekorBundle []byte
}

// Install syncs req.Registry, checks each requested item against the trust
// gate, fetches and verifies the items that pass, stages them into the
// Library, and installs them to req.Targets.
//
// A choice it needs from the user comes back as *lifecycle.DecisionRequired
// before anything is fetched, except OverwritePinned, which comes after the
// fetch and before anything is staged. Re-invoke with the answer in
// req.Decisions. An item that fails stops alone, with ItemResult.Err set;
// the error return is for failures of the whole call. A ctx cancelled
// before staging stages nothing; one cancelled after staging skips the
// provider installs.
func (o *Operation) Install(ctx context.Context, req Request) (Result, error) {
	var res Result
	now := time.Now
	if o.Now != nil {
		now = o.Now
	}
	start := now()

	rootInfo := moat.BundledTrustedRoot(start)
	if rootInfo.Status == moat.TrustedRootStatusExpired ||
		rootInfo.Status == moat.TrustedRootStatusMissing ||
		rootInfo.Status == moat.TrustedRootStatusCorrupt {
		return res, output.NewStructuredErrorDetail(
			output.ErrMoatTrustedRootStale,
			fmt.Sprintf("bundled trusted root unusable while installing from registry %q", req.Registry),
			"Run `syllago update` to refresh the bundled Sigstore trusted root.",
			rootInfo.Status.String(),
		)
	}

	reg, manifest, err := o.syncRegistry(ctx, req, start, &res)
	if err != nil {
		return res, err
	}
	res.Manifest = manifest
	lockfilePath := moat.LockfilePath(req.ProjectRoot)
	passed, err := o.gateItems(req, reg, manifest, lockfilePath, &res)
	if err != nil || req.DryRun {
		return res, err
	}

	// The fetch runs outside the install lock: it is network work, and it
	// writes only the source cache.
	clone := o.Clone
	if clone == nil {
		clone = moat.CloneRepoFn
	}
	clones := map[string]string{}
	defer func() {
		for _, dir := range clones {
			_ = os.RemoveAll(dir)
		}
	}()
	var ready []fetched
	for _, i := range passed {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		ir := &res.Items[i]
		f, err := fetchItem(ctx, clone, clones, ir.Entry, reg.Name, &manifest.RegistrySigningProfile, rootInfo.Bytes)
		if err != nil {
			ir.Err = err
			continue
		}
		f.idx = i
		ready = append(ready, f)
	}
	if err := ctx.Err(); err != nil {
		return res, err
	}
	if len(ready) == 0 {
		return res, nil
	}

	lc := o.Lifecycle
	if lc == nil {
		lc = lifecycle.New()
	}
	if err := stageItems(ctx, lc, req, reg, lockfilePath, ready, start, &res); err != nil {
		return res, err
	}
	if len(req.Targets) > 0 {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		installItems(ctx, lc, req, reg, ready, start, &res)
		for _, ir := range res.Items {
			if errors.Is(ir.Err, context.Canceled) {
				return res, ir.Err
			}
		}
	}
	return res, nil
}

// syncRegistry syncs req.Registry and returns its current manifest, or the
// error or decision that stops the install before any item is checked.
func (o *Operation) syncRegistry(ctx context.Context, req Request, start time.Time, res *Result) (*config.Registry, *moat.Manifest, error) {
	reg, err := lookupRegistry(req.Registry)
	if err != nil {
		return nil, nil, err
	}

	cacheDir, err := config.GlobalDirPath()
	if err != nil {
		return nil, nil, output.NewStructuredError(output.ErrSystemHomedir, "cannot determine the syllago config directory", "Set the HOME environment variable")
	}
	sync := o.Sync
	if sync == nil {
		sync = registryops.SyncOne
	}
	synced, err := sync(ctx, reg.Name, registryops.SyncOpts{
		AcceptTOFU:   req.Decisions.AcceptTOFU,
		LockfileRoot: req.ProjectRoot,
		CacheDir:     cacheDir,
		Now:          start,
	})
	res.Sync = synced
	if err != nil {
		// Sync classifies expiry only after the manifest verified, and it
		// saves an expired manifest like any other, so a failed save still
		// reports the expiry.
		if synced.MoatResult.Staleness == moat.StalenessExpired {
			return nil, nil, ErrManifestExpired
		}
		return nil, nil, &SyncError{Registry: reg.Name, Err: err}
	}
	if synced.GateProfileChanged {
		return nil, nil, ErrProfileChanged
	}
	if synced.GateTOFUNeeded {
		return nil, nil, &lifecycle.DecisionRequired{Kind: lifecycle.TrustOnFirstUse, Context: synced.MoatResult.IncomingProfile}
	}

	// A 304 returns no manifest body: the copy cached by the sync that
	// fetched it is still the current one. Sync classified the 304 without
	// that body, so its expiry is checked here.
	manifest := synced.MoatResult.Manifest
	res.Staleness = synced.MoatResult.Staleness
	if synced.MoatResult.NotModified {
		manifest, err = regdiff.LoadCachedManifest(cacheDir, reg.Name)
		if err != nil || manifest == nil {
			return nil, nil, output.NewStructuredError(
				output.ErrMoatInvalid,
				fmt.Sprintf("registry %q is at the pinned revision but the manifest body is not cached locally", reg.Name),
				"Clear the cached ETag by removing the `manifest_etag` field for registry \""+reg.Name+"\" from ~/.syllago/config.json, then run `syllago registry sync "+reg.Name+"` to force a full re-fetch.",
			)
		}
		res.Staleness = moat.CheckStaleness(synced.MoatResult.FetchedAt, manifest.Expires, start)
	}
	if res.Staleness == moat.StalenessExpired {
		return nil, nil, ErrManifestExpired
	}
	return reg, manifest, nil
}

// gateItems checks each requested item against the trust gate and returns
// the indexes in res.Items of those that passed. A warning still waiting on
// the user comes back as *lifecycle.DecisionRequired.
func (o *Operation) gateItems(req Request, reg *config.Registry, manifest *moat.Manifest, lockfilePath string, res *Result) ([]int, error) {
	lf, err := moat.LoadLockfile(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}
	revSet := moat.NewRevocationSet()
	revSet.AddFromManifest(manifest, reg.ManifestURI)
	session := req.Session
	if session == nil {
		session = moat.NewSession()
	}

	var publisherPrompts, privatePrompts []GatePrompt
	var passed []int
	for _, item := range req.Items {
		res.Items = append(res.Items, ItemResult{Name: item.Name})
		ir := &res.Items[len(res.Items)-1]
		entry, ok := findEntry(manifest, item)
		if !ok {
			what := fmt.Sprintf("an item named %q", item.Name)
			if item.Type != "" {
				what = fmt.Sprintf("a %s named %q", item.Type.Label(), item.Name)
			}
			ir.Err = output.NewStructuredError(
				output.ErrInstallItemNotFound,
				fmt.Sprintf("registry %q does not list %s in its manifest", reg.Name, what),
				"Run `syllago registry items "+reg.Name+"` to see available content.",
			)
			continue
		}
		ir.Entry = entry
		// A refusal stands even when a newer manifest no longer asks.
		if req.Decisions.refused(entry.ContentHash) {
			ir.Err = ErrDeclined
			continue
		}

		gate := installer.PreInstallCheck(entry, reg.ManifestURI, lf, revSet, session, o.MinTier)
		ir.Gate = gate
		for ir.Err == nil {
			switch gate.Decision {
			case installer.MOATGatePublisherWarn:
				ok, decided := req.Decisions.PublisherWarn[entry.ContentHash]
				if !decided {
					publisherPrompts = append(publisherPrompts, GatePrompt{Entry: entry, Gate: gate})
					break
				}
				if !ok {
					ir.Err = ErrDeclined
					break
				}
				installer.MarkPublisherConfirmed(session, reg.ManifestURI, entry.ContentHash)
				gate = installer.PreInstallCheck(entry, reg.ManifestURI, lf, revSet, session, o.MinTier)
				continue
			case installer.MOATGatePrivatePrompt:
				ok, decided := req.Decisions.PrivateSource[entry.ContentHash]
				if !decided {
					privatePrompts = append(privatePrompts, GatePrompt{Entry: entry, Gate: gate})
					break
				}
				if !ok {
					ir.Err = ErrDeclined
					break
				}
				installer.MarkPrivateConfirmed(session, reg.ManifestURI, entry.ContentHash)
				gate = installer.PreInstallCheck(entry, reg.ManifestURI, lf, revSet, session, o.MinTier)
				continue
			case installer.MOATGateProceed:
				// Targets are checked after the gate, so a trust warning is
				// reported whatever the targets, and a dry run checks none.
				if !req.DryRun {
					ir.Err = checkTargets(ir, reg.Name, req.Targets)
				}
				if ir.Err == nil {
					passed = append(passed, len(res.Items)-1)
				}
			default:
				ir.Err = gateError(entry, gate)
			}
			break
		}
	}
	if len(publisherPrompts) > 0 {
		return nil, &lifecycle.DecisionRequired{Kind: lifecycle.PublisherWarn, Context: publisherPrompts}
	}
	if len(privatePrompts) > 0 {
		return nil, &lifecycle.DecisionRequired{Kind: lifecycle.PrivateSource, Context: privatePrompts}
	}
	return passed, nil
}

// stageItems records the fetched items in the lockfile and stages them into
// the Library under one lc.Overwrite. An item the user declined to
// overwrite gets ErrDeclined.
func stageItems(ctx context.Context, lc *lifecycle.Module, req Request, reg *config.Registry, lockfilePath string, ready []fetched, start time.Time, res *Result) error {
	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}
	byPath := make(map[string]fetched, len(ready))
	dests := make([]lifecycle.Destination, 0, len(ready))
	for _, f := range ready {
		entry := res.Items[f.idx].Entry
		ct, _ := moat.FromMOATType(entry.Type)
		d := lifecycle.Destination{Type: ct, Name: entry.Name, Path: filepath.Join(globalDir, string(ct), entry.Name)}
		byPath[d.Path] = f
		dests = append(dests, d)
	}
	staged := map[int]bool{}
	var err error
	res.Stage, err = lc.Overwrite(lifecycle.OverwriteRequest{
		Destinations: dests,
		Decisions:    req.Decisions.Decisions,
		// Write runs under the install lock, so the lockfile entry and the
		// Library copy land together, and a pinned item the user declined
		// gets neither.
		Write: func(approved []lifecycle.Destination) ([]lifecycle.Written, error) {
			// The wait for the lock can outlast a cancel.
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			lf, err := moat.LoadLockfile(lockfilePath)
			if err != nil {
				return nil, fmt.Errorf("load lockfile: %w", err)
			}
			var recorded []lifecycle.Destination
			for _, d := range approved {
				f := byPath[d.Path]
				ir := &res.Items[f.idx]
				if err := recordEntry(lf, ir.Entry, reg.Name, reg.ManifestURI, f.rekorBundle, start); err != nil {
					ir.Err = err
					continue
				}
				recorded = append(recorded, d)
			}
			if len(recorded) == 0 {
				return nil, nil
			}
			if err := lf.Save(lockfilePath); err != nil {
				err = output.NewStructuredErrorDetail(
					output.ErrMoatInvalid,
					"could not save moat lockfile",
					"Check filesystem permissions on .syllago/moat-lockfile.json.",
					err.Error(),
				)
				for _, d := range recorded {
					res.Items[byPath[d.Path].idx].Err = err
				}
				return nil, nil
			}
			var written []lifecycle.Written
			for _, d := range recorded {
				f := byPath[d.Path]
				ir := &res.Items[f.idx]
				item, prev, err := StageIntoLibraryKeepPrev(f.cacheDir, ir.Entry, reg.Name, globalDir, start)
				written = append(written, lifecycle.Written{Path: d.Path, PreviousCopy: prev})
				if err != nil {
					ir.Err = stageError(reg.Name, ir.Entry, err)
					continue
				}
				ir.Library = item
				staged[f.idx] = true
			}
			return written, nil
		},
	})
	if err != nil {
		return err
	}
	for _, f := range ready {
		ir := &res.Items[f.idx]
		if ir.Err == nil && !staged[f.idx] {
			ir.Err = ErrDeclined
		}
	}
	return nil
}

// installItems installs each staged item to the targets that support it.
func installItems(ctx context.Context, lc *lifecycle.Module, req Request, reg *config.Registry, ready []fetched, start time.Time, res *Result) {
	for _, f := range ready {
		ir := &res.Items[f.idx]
		if ir.Err != nil {
			continue
		}
		targets := supportedTargets(ir, req.Targets)
		out, err := lc.Install(lifecycle.InstallRequest{
			Context:     ctx,
			Item:        ir.Library,
			ProjectRoot: req.ProjectRoot,
			Targets:     targets,
			Method:      req.Method,
			Scan:        req.Scan,
			Frozen:      req.Frozen,
			Provenance: &installstore.MOATProvenance{
				ManifestURI: reg.ManifestURI,
				SourceURI:   ir.Entry.SourceURI,
				TrustTier:   ir.Entry.TrustTier().String(),
				AttestedAt:  start,
			},
		})
		ir.Install = out
		switch {
		case err == nil:
		case len(out.Unattempted) > 0:
			// The install lock was unavailable or the install was
			// cancelled; report it as it is.
			ir.Err = err
		default:
			ir.Err = output.NewStructuredErrorDetail(
				output.ErrInstallNotWritable,
				fmt.Sprintf("could not install %s/%s", reg.Name, ir.Entry.Name),
				"The source artifact was fetched and verified but the provider-side install failed. Check filesystem permissions, that the target directory is writable, and that the provider supports this content type.",
				err.Error(),
			)
		}
	}
}

// findEntry finds item in the manifest, matching its type when it has one.
func findEntry(m *moat.Manifest, item Item) (*moat.ContentEntry, bool) {
	if item.Type == "" {
		return moat.FindContentEntry(m, item.Name)
	}
	want, ok := moat.ToMOATType(item.Type)
	if m == nil || !ok {
		return nil, false
	}
	for i := range m.Content {
		if e := &m.Content[i]; e.Name == item.Name && e.Type == want {
			return e, true
		}
	}
	return nil, false
}

// lookupRegistry finds name in the global config, where `registry add`
// saves every registry and where the sync reads it.
func lookupRegistry(name string) (*config.Registry, error) {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return nil, output.NewStructuredErrorDetail(output.ErrConfigInvalid, "loading global config", "Check ~/.syllago/config.json syntax", err.Error())
	}
	for i := range cfg.Registries {
		if cfg.Registries[i].Name != name {
			continue
		}
		reg := &cfg.Registries[i]
		if !reg.IsMOAT() {
			return nil, output.NewStructuredError(
				output.ErrMoatInvalid,
				fmt.Sprintf("registry %q is not MOAT-backed; <registry>/<item> install syntax requires a MOAT registry", name),
				"Use the plain 'syllago install <item>' form for git-registry content, or configure this registry as MOAT with a manifest_uri.",
			)
		}
		return reg, nil
	}
	return nil, output.NewStructuredError(
		output.ErrRegistryNotFound,
		fmt.Sprintf("registry %q not found in config", name),
		"Run 'syllago registry list' to see configured registries, or 'syllago registry add' to add one.",
	)
}

// checkTargets records the targets that cannot take ir's type, and fails
// the item when its type is not one syllago installs or no target takes it.
func checkTargets(ir *ItemResult, regName string, targets []lifecycle.Target) error {
	entry := ir.Entry
	ct, ok := moat.FromMOATType(entry.Type)
	if _, dirOK := moat.CategoryDirForMOATType(entry.Type); !ok || !dirOK {
		return output.NewStructuredError(
			output.ErrMoatInvalid,
			fmt.Sprintf("unsupported MOAT content type %q for %s/%s", entry.Type, regName, entry.Name),
			"Only skill, agent, rules, and command are normative MOAT types. hook and mcp are deferred.",
		)
	}
	for _, t := range targets {
		if t.Provider.SupportsType != nil && !t.Provider.SupportsType(ct) {
			ir.Unsupported = append(ir.Unsupported, t)
		}
	}
	if len(targets) > 0 && len(ir.Unsupported) == len(targets) {
		names := make([]string, 0, len(targets))
		for _, t := range targets {
			names = append(names, t.Provider.Name)
		}
		return output.NewStructuredError(
			output.ErrInstallNotWritable,
			fmt.Sprintf("could not install %s/%s: %s does not support %s", regName, entry.Name, joinNames(names), ct.Label()),
			"Choose a provider that supports this content type.",
		)
	}
	return nil
}

func supportedTargets(ir *ItemResult, targets []lifecycle.Target) []lifecycle.Target {
	skip := make(map[string]bool, len(ir.Unsupported))
	for _, t := range ir.Unsupported {
		skip[t.Provider.Slug] = true
	}
	var out []lifecycle.Target
	for _, t := range targets {
		if !skip[t.Provider.Slug] {
			out = append(out, t)
		}
	}
	return out
}

func joinNames(names []string) string {
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	}
	s := ""
	for i, n := range names {
		switch {
		case i == len(names)-1:
			s += ", and " + n
		case i > 0:
			s += ", " + n
		default:
			s = n
		}
	}
	return s
}

// gateError is the item error for a gate that refuses outright.
func gateError(entry *moat.ContentEntry, gate installer.GateBlock) error {
	switch gate.Decision {
	case installer.MOATGateHardBlock:
		reason := ""
		if gate.Revocation != nil {
			reason = moat.SanitizeForDisplay(gate.Revocation.Reason)
		}
		return output.NewStructuredErrorDetail(
			output.ErrMoatRevocationBlock,
			fmt.Sprintf("registry-source revocation refuses install of %q", entry.Name),
			"Registry-source revocations are permanent. Contact the publisher or choose an alternative item.",
			"reason="+reason,
		)
	case installer.MOATGateTierBelowPolicy:
		return output.NewStructuredErrorDetail(
			output.ErrMoatTierBelowPolicy,
			fmt.Sprintf("trust tier of %q (%s) is below configured minimum (%s)",
				entry.Name, gate.ObservedTier.String(), gate.MinTier.String()),
			"Raise the item's trust tier (ask the publisher to sign / dual-attest), or lower the install-gate minimum.",
			fmt.Sprintf("observed=%s min=%s", gate.ObservedTier.String(), gate.MinTier.String()),
		)
	}
	return fmt.Errorf("moatinstall: unhandled gate decision %v", gate.Decision)
}

func stageError(regName string, entry *moat.ContentEntry, err error) error {
	var structured output.StructuredError
	if errors.As(err, &structured) {
		return err
	}
	return output.NewStructuredErrorDetail(
		output.ErrInstallNotWritable,
		fmt.Sprintf("could not stage %s/%s into the global library", regName, entry.Name),
		"The source artifact was fetched and verified but could not be added to the global library. Check filesystem permissions in ~/.syllago/content/ and ensure the registry owns any existing item at that path.",
		err.Error(),
	)
}

// fetchItem verifies entry's attestations, clones its source repository
// once per call into clones, and copies the item into the source cache.
func fetchItem(ctx context.Context, clone moat.CloneRepoFunc, clones map[string]string, entry *moat.ContentEntry, regName string, registryProfile *moat.SigningProfile, trustedRootJSON []byte) (fetched, error) {
	rekorBundle, err := verifyAttestations(ctx, entry, regName, registryProfile, trustedRootJSON)
	if err != nil {
		return fetched{}, err
	}
	categoryDir, err := checkSource(entry, regName)
	if err != nil {
		return fetched{}, err
	}
	cloneDir, ok := clones[entry.SourceURI]
	if !ok {
		dir, _, err := cloneSource(ctx, clone, entry, regName)
		if err != nil {
			return fetched{}, err
		}
		clones[entry.SourceURI] = dir
		cloneDir = dir
	}
	cacheDir, err := extractItem(cloneDir, categoryDir, entry, regName)
	if err != nil {
		return fetched{}, err
	}
	return fetched{cacheDir: cacheDir, rekorBundle: rekorBundle}, nil
}

// refused reports whether the user declined a warning about the content
// at hash.
func (d Decisions) refused(hash string) bool {
	if ok, decided := d.PublisherWarn[hash]; decided && !ok {
		return true
	}
	ok, decided := d.PrivateSource[hash]
	return decided && !ok
}
