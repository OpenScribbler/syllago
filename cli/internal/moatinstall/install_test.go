package moatinstall

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
	"github.com/OpenScribbler/syllago/cli/internal/syllagolock"
)

// opEnv is an Operation over registry "example", whose items are skills in
// one source repository. Every path it writes is a temp dir.
type opEnv struct {
	op        *Operation
	manifest  *moat.Manifest
	fixture   string
	project   string
	library   string
	configDir string
	now       time.Time
	clones    int
	syncs     []registryops.SyncOpts
	// sync answers each sync; nil means a fresh, verified manifest.
	sync func(opts registryops.SyncOpts) (registryops.SyncOutcome, error)
}

func newOpEnv(t *testing.T, names ...string) *opEnv {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	e := &opEnv{
		fixture:   t.TempDir(),
		project:   t.TempDir(),
		library:   t.TempDir(),
		configDir: t.TempDir(),
		now:       time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC),
	}
	origConfig, origLibrary := config.GlobalDirOverride, catalog.GlobalContentDirOverride
	config.GlobalDirOverride, catalog.GlobalContentDirOverride = e.configDir, e.library
	PreviousRootOverride = t.TempDir()
	t.Cleanup(func() {
		config.GlobalDirOverride, catalog.GlobalContentDirOverride = origConfig, origLibrary
		PreviousRootOverride = ""
	})
	stubSourceCacheDir(t)
	stubCloneScratchDir(t)
	if err := config.SaveGlobal(&config.Config{Registries: []config.Registry{{
		Name: "example", URL: "https://example.com/m", Type: config.RegistryTypeMOAT, ManifestURI: "https://example.com/m",
	}}}); err != nil {
		t.Fatal(err)
	}

	e.manifest = &moat.Manifest{}
	for _, name := range names {
		e.setItem(t, name, "# "+name+"\n")
	}
	e.op = &Operation{
		Clone: func(_ context.Context, _, dest string) error {
			e.clones++
			return moat.CopyTree(e.fixture, dest)
		},
		Sync: func(_ context.Context, _ string, opts registryops.SyncOpts) (registryops.SyncOutcome, error) {
			e.syncs = append(e.syncs, opts)
			if e.sync != nil {
				return e.sync(opts)
			}
			return registryops.SyncOutcome{MoatResult: moat.SyncResult{
				Manifest: e.manifest, Staleness: moat.StalenessFresh, FetchedAt: e.now,
			}}, nil
		},
		Now: func() time.Time { return e.now },
	}
	return e
}

// setItem writes name's content into the source repository and lists it in
// the manifest at that content's hash.
func (e *opEnv) setItem(t *testing.T, name, content string) *moat.ContentEntry {
	t.Helper()
	hash := makeRepoFixture(t, e.fixture, "skills", name, map[string]string{"SKILL.md": content})
	for i := range e.manifest.Content {
		if e.manifest.Content[i].Name == name {
			e.manifest.Content[i].ContentHash = hash
			return &e.manifest.Content[i]
		}
	}
	e.manifest.Content = append(e.manifest.Content, moat.ContentEntry{
		Name: name, DisplayName: name, Type: "skill", ContentHash: hash, SourceURI: fakeRepoURL, AttestedAt: e.now,
	})
	return &e.manifest.Content[len(e.manifest.Content)-1]
}

func (e *opEnv) request(names ...string) Request {
	items := make([]Item, len(names))
	for i, name := range names {
		items[i] = Item{Name: name}
	}
	return Request{Registry: "example", Items: items, ProjectRoot: e.project}
}

func (e *opEnv) inLibrary(name string) bool {
	_, err := os.Stat(filepath.Join(e.library, string(catalog.Skills), name, "SKILL.md"))
	return err == nil
}

func (e *opEnv) lockedHashes(t *testing.T, name string) []string {
	t.Helper()
	lf, err := moat.LoadLockfile(moat.LockfilePath(e.project))
	if err != nil {
		t.Fatal(err)
	}
	var hashes []string
	for _, entry := range lf.Entries {
		if entry.Name == name {
			hashes = append(hashes, entry.ContentHash)
		}
	}
	return hashes
}

func requireDecision(t *testing.T, err error, kind lifecycle.DecisionKind) *lifecycle.DecisionRequired {
	t.Helper()
	var dr *lifecycle.DecisionRequired
	if !errors.As(err, &dr) || dr.Kind != kind {
		t.Fatalf("err = %v, want a %s decision", err, kind)
	}
	return dr
}

func TestInstall_StagesIntoLibrary(t *testing.T) {
	e := newOpEnv(t, "a")
	res, err := e.op.Install(context.Background(), e.request("a"))
	if err != nil || res.Items[0].Err != nil {
		t.Fatalf("Install: err=%v item=%v", err, res.Items[0].Err)
	}
	if !e.inLibrary("a") {
		t.Error("a is not in the Library")
	}
	if got := e.lockedHashes(t, "a"); len(got) != 1 || got[0] != e.manifest.Content[0].ContentHash {
		t.Errorf("lockfile hashes = %v, want the manifest's", got)
	}
}

func TestInstall_DryRunFetchesNothing(t *testing.T) {
	e := newOpEnv(t, "a")
	req := e.request("a")
	req.DryRun = true
	res, err := e.op.Install(context.Background(), req)
	if err != nil || res.Items[0].Err != nil {
		t.Fatalf("Install: err=%v item=%v", err, res.Items[0].Err)
	}
	if e.clones != 0 || e.inLibrary("a") || len(e.lockedHashes(t, "a")) != 0 {
		t.Errorf("dry run cloned %d times, staged=%v, locked=%v", e.clones, e.inLibrary("a"), e.lockedHashes(t, "a"))
	}
}

// A request naming no items lists what the registry offers and fetches
// nothing.
func TestInstall_NoItemsReturnsManifest(t *testing.T) {
	e := newOpEnv(t, "a", "b")
	req := e.request()
	req.DryRun = true
	res, err := e.op.Install(context.Background(), req)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Manifest == nil || len(res.Manifest.Content) != 2 {
		t.Fatalf("Manifest = %+v, want both items", res.Manifest)
	}
	if e.clones != 0 || len(res.Items) != 0 {
		t.Errorf("cloned %d times, items=%v; want neither", e.clones, res.Items)
	}
}

// An item no target can take fails before its source is downloaded.
func TestInstall_UnsupportedTargetFailsBeforeFetch(t *testing.T) {
	e := newOpEnv(t, "a")
	req := e.request("a")
	req.Targets = []lifecycle.Target{{Provider: provider.Provider{
		Name: "Rules Only", Slug: "rules-only",
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Rules },
	}}}
	res, err := e.op.Install(context.Background(), req)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Items[0].Err == nil {
		t.Fatal("want the item refused for its type")
	}
	if e.clones != 0 || e.inLibrary("a") {
		t.Errorf("cloned %d times, staged=%v; want neither", e.clones, e.inLibrary("a"))
	}
}

func TestInstall_TrustOnFirstUse(t *testing.T) {
	e := newOpEnv(t, "a")
	e.sync = func(opts registryops.SyncOpts) (registryops.SyncOutcome, error) {
		out := registryops.SyncOutcome{MoatResult: moat.SyncResult{Manifest: e.manifest, FetchedAt: e.now}}
		out.GateTOFUNeeded = !opts.AcceptTOFU
		return out, nil
	}
	_, err := e.op.Install(context.Background(), e.request("a"))
	requireDecision(t, err, lifecycle.TrustOnFirstUse)
	if e.clones != 0 {
		t.Errorf("cloned before trust was decided")
	}

	req := e.request("a")
	req.Decisions.AcceptTOFU = true
	res, err := e.op.Install(context.Background(), req)
	if err != nil || res.Items[0].Err != nil {
		t.Fatalf("Install after accepting: err=%v item=%v", err, res.Items[0].Err)
	}
	if !e.syncs[1].AcceptTOFU || !e.inLibrary("a") {
		t.Errorf("AcceptTOFU=%v staged=%v, want both", e.syncs[1].AcceptTOFU, e.inLibrary("a"))
	}
}

func TestInstall_PublisherWarn(t *testing.T) {
	e := newOpEnv(t, "a")
	e.manifest.Revocations = []moat.Revocation{{ContentHash: e.manifest.Content[0].ContentHash, Reason: "deprecated", Source: "publisher"}}

	_, err := e.op.Install(context.Background(), e.request("a"))
	dr := requireDecision(t, err, lifecycle.PublisherWarn)
	if prompts, _ := dr.Context.([]GatePrompt); len(prompts) != 1 || prompts[0].Entry.Name != "a" {
		t.Fatalf("context = %+v, want one prompt for a", dr.Context)
	}

	declined := e.request("a")
	declined.Decisions.PublisherWarn = map[string]bool{e.manifest.Content[0].ContentHash: false}
	res, err := e.op.Install(context.Background(), declined)
	if err != nil || !errors.Is(res.Items[0].Err, ErrDeclined) {
		t.Fatalf("declined: err=%v item=%v, want ErrDeclined", err, res.Items[0].Err)
	}
	if e.clones != 0 {
		t.Errorf("a declined item was cloned")
	}

	confirmed := e.request("a")
	confirmed.Decisions.PublisherWarn = map[string]bool{e.manifest.Content[0].ContentHash: true}
	res, err = e.op.Install(context.Background(), confirmed)
	if err != nil || res.Items[0].Err != nil || !e.inLibrary("a") {
		t.Fatalf("confirmed: err=%v item=%v staged=%v", err, res.Items[0].Err, e.inLibrary("a"))
	}
}

// Confirming a publisher warning leaves a private source still to decide.
func TestInstall_PrivateSourceDecidedSeparately(t *testing.T) {
	e := newOpEnv(t, "a")
	e.manifest.Content[0].PrivateRepo = true
	e.manifest.Revocations = []moat.Revocation{{ContentHash: e.manifest.Content[0].ContentHash, Reason: "deprecated", Source: "publisher"}}

	req := e.request("a")
	req.Decisions.PublisherWarn = map[string]bool{e.manifest.Content[0].ContentHash: true}
	_, err := e.op.Install(context.Background(), req)
	requireDecision(t, err, lifecycle.PrivateSource)

	req.Decisions.PrivateSource = map[string]bool{e.manifest.Content[0].ContentHash: true}
	res, err := e.op.Install(context.Background(), req)
	if err != nil || res.Items[0].Err != nil || !e.inLibrary("a") {
		t.Fatalf("err=%v item=%v staged=%v", err, res.Items[0].Err, e.inLibrary("a"))
	}
}

func TestInstall_ItemsFromOneRepoCloneOnce(t *testing.T) {
	e := newOpEnv(t, "a", "b")
	res, err := e.op.Install(context.Background(), e.request("a", "b"))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	for _, ir := range res.Items {
		if ir.Err != nil {
			t.Errorf("%s: %v", ir.Name, ir.Err)
		}
	}
	if e.clones != 1 || !e.inLibrary("a") || !e.inLibrary("b") {
		t.Errorf("cloned %d times, staged a=%v b=%v; want 1 clone and both", e.clones, e.inLibrary("a"), e.inLibrary("b"))
	}
}

func TestInstall_CancelledStagesNothing(t *testing.T) {
	e := newOpEnv(t, "a")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.op.Install(ctx, e.request("a")); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if e.inLibrary("a") || len(e.lockedHashes(t, "a")) != 0 {
		t.Errorf("staged=%v locked=%v after cancel", e.inLibrary("a"), e.lockedHashes(t, "a"))
	}
}

// A pinned item the user declines to overwrite keeps its Library copy and
// gains no lockfile entry for the version it did not take.
func TestInstall_DeclinedPinnedItemNotRecorded(t *testing.T) {
	e := newOpEnv(t, "a")
	res, err := e.op.Install(context.Background(), e.request("a"))
	if err != nil || res.Items[0].Err != nil {
		t.Fatalf("first Install: err=%v item=%v", err, res.Items[0].Err)
	}
	v1 := e.manifest.Content[0].ContentHash
	storePath := filepath.Join(e.configDir, "installs.json")
	coord := installstore.Coord{Registry: "example", Type: string(catalog.Skills), Name: "a"}
	if err := installstore.RecordInstallMeta(storePath, coord, res.Items[0].Library.Path, installstore.PlacementInput{
		Provider: "claude-code", Mechanism: installstore.MechanismSymlink, Path: filepath.Join(t.TempDir(), "a"),
	}, installstore.InstallMeta{}, e.now); err != nil {
		t.Fatal(err)
	}
	if err := installstore.SetPinned(storePath, coord, true, e.now); err != nil {
		t.Fatal(err)
	}

	e.setItem(t, "a", "# a v2\n")
	_, err = e.op.Install(context.Background(), e.request("a"))
	dr := requireDecision(t, err, lifecycle.OverwritePinned)
	pinned := dr.Context.([]lifecycle.PinnedDestination)

	req := e.request("a")
	req.Decisions.Overwrite = map[string]bool{pinned[0].Path: false}
	res, err = e.op.Install(context.Background(), req)
	if err != nil || !errors.Is(res.Items[0].Err, ErrDeclined) {
		t.Fatalf("declined: err=%v item=%v, want ErrDeclined", err, res.Items[0].Err)
	}
	if got := e.lockedHashes(t, "a"); len(got) != 1 || got[0] != v1 {
		t.Errorf("lockfile hashes = %v, want only %s", got, v1)
	}
}

// Regression: a sync that verified an expired manifest and then failed to
// save it reported the save failure instead of the expiry.
func TestInstall_ExpiredManifestSaveFailure(t *testing.T) {
	e := newOpEnv(t, "a")
	e.sync = func(registryops.SyncOpts) (registryops.SyncOutcome, error) {
		return registryops.SyncOutcome{MoatResult: moat.SyncResult{
			Manifest: e.manifest, Staleness: moat.StalenessExpired, FetchedAt: e.now,
		}}, errors.New("save global config: permission denied")
	}
	if _, err := e.op.Install(context.Background(), e.request("a")); !errors.Is(err, ErrManifestExpired) {
		t.Fatalf("err = %v, want ErrManifestExpired", err)
	}
}

// Regression: a refusal was dropped when the next sync's manifest no longer
// carried the warning, and the item installed.
func TestInstall_RefusalStandsWhenWarningGoes(t *testing.T) {
	e := newOpEnv(t, "a")
	hash := e.manifest.Content[0].ContentHash
	req := e.request("a")
	req.Decisions.PublisherWarn = map[string]bool{hash: false}
	res, err := e.op.Install(context.Background(), req)
	if err != nil || !errors.Is(res.Items[0].Err, ErrDeclined) {
		t.Fatalf("err=%v item=%v, want ErrDeclined", err, res.Items[0].Err)
	}
	if e.inLibrary("a") {
		t.Error("a refused item was staged")
	}
}

// Regression: an answer keyed by item name approved new content published
// under the same name.
func TestInstall_ApprovalCoversOnlyTheContentShown(t *testing.T) {
	e := newOpEnv(t, "a")
	approved := e.manifest.Content[0].ContentHash
	e.setItem(t, "a", "# a v2\n")
	e.manifest.Revocations = []moat.Revocation{{ContentHash: e.manifest.Content[0].ContentHash, Reason: "deprecated", Source: "publisher"}}
	req := e.request("a")
	req.Decisions.PublisherWarn = map[string]bool{approved: true}
	_, err := e.op.Install(context.Background(), req)
	requireDecision(t, err, lifecycle.PublisherWarn)
	if e.clones != 0 {
		t.Error("cloned content the user had not approved")
	}
}

// Regression: a cancel while Install waited for the install lock still
// wrote the Library and the lockfile once the lock came free.
func TestInstall_CancelWhileWaitingForLockStagesNothing(t *testing.T) {
	e := newOpEnv(t, "a")
	release, err := syllagolock.Acquire(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := e.op.Install(ctx, e.request("a"))
		done <- err
	}()
	time.Sleep(300 * time.Millisecond) // long enough to reach the lock
	cancel()
	release()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if e.inLibrary("a") || len(e.lockedHashes(t, "a")) != 0 {
		t.Errorf("staged=%v locked=%v after cancel", e.inLibrary("a"), e.lockedHashes(t, "a"))
	}
}

// Regression: an unsupported target turned a trust warning into a target
// error, and a dry run refused to preview the item.
func TestInstall_GateBeforeTargets(t *testing.T) {
	rulesOnly := []lifecycle.Target{{Provider: provider.Provider{
		Name: "Rules Only", Slug: "rules-only",
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Rules },
	}}}
	e := newOpEnv(t, "a")
	e.manifest.Revocations = []moat.Revocation{{ContentHash: e.manifest.Content[0].ContentHash, Reason: "deprecated", Source: "publisher"}}
	req := e.request("a")
	req.Targets = rulesOnly
	_, err := e.op.Install(context.Background(), req)
	requireDecision(t, err, lifecycle.PublisherWarn)

	e = newOpEnv(t, "a")
	req = e.request("a")
	req.Targets = rulesOnly
	req.DryRun = true
	res, err := e.op.Install(context.Background(), req)
	if err != nil || res.Items[0].Err != nil {
		t.Fatalf("dry run: err=%v item=%v", err, res.Items[0].Err)
	}
}

// One item's failure leaves the others to install.
func TestInstall_ItemFailsAlone(t *testing.T) {
	e := newOpEnv(t, "a")
	res, err := e.op.Install(context.Background(), e.request("missing", "a"))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Items[0].Err == nil || res.Items[1].Err != nil || !e.inLibrary("a") {
		t.Errorf("missing err=%v, a err=%v staged=%v", res.Items[0].Err, res.Items[1].Err, e.inLibrary("a"))
	}
}

// Regression: a manifest listing one name under two types gave the caller
// the first one listed, whatever type it asked for.
func TestInstall_TypedItemTakesItsType(t *testing.T) {
	e := newOpEnv(t, "shared")
	hash := makeRepoFixture(t, e.fixture, "agents", "shared", map[string]string{"AGENT.md": "# agent\n"})
	e.manifest.Content = append(e.manifest.Content, moat.ContentEntry{
		Name: "shared", Type: "agent", ContentHash: hash, SourceURI: fakeRepoURL, AttestedAt: e.now,
	})
	req := e.request()
	req.Items = []Item{{Name: "shared", Type: catalog.Agents}}
	res, err := e.op.Install(context.Background(), req)
	if err != nil || res.Items[0].Err != nil {
		t.Fatalf("Install: err=%v item=%v", err, res.Items[0].Err)
	}
	if got := res.Items[0].Entry.Type; got != "agent" {
		t.Errorf("entry type = %q, want agent", got)
	}
	if e.inLibrary("shared") {
		t.Error("the skill reached the Library")
	}
	if _, err := os.Stat(filepath.Join(e.library, string(catalog.Agents), "shared", "AGENT.md")); err != nil {
		t.Errorf("the agent is not in the Library: %v", err)
	}
}

// Regression: staging that replaced the Library copy and then failed left
// the install record describing the replaced version with no Previous.
func TestInstall_FailedStageStillRotatesRecord(t *testing.T) {
	e := newOpEnv(t, "my-skill")
	ctx := context.Background()
	if res, err := e.op.Install(ctx, e.request("my-skill")); err != nil || res.Items[0].Err != nil {
		t.Fatalf("install v1: err=%v item=%v", err, res.Items[0].Err)
	}
	libPath := filepath.Join(e.library, string(catalog.Skills), "my-skill")
	storePath := filepath.Join(e.configDir, "installs.json")
	coord := installstore.Coord{Registry: "example", Type: string(catalog.Skills), Name: "my-skill"}
	if err := installstore.RecordInstallMeta(storePath, coord, libPath, installstore.PlacementInput{
		Provider: "claude-code", Mechanism: installstore.MechanismSymlink, Path: filepath.Join(t.TempDir(), "my-skill"),
	}, installstore.InstallMeta{}, e.now); err != nil {
		t.Fatal(err)
	}
	v1, err := installstore.HashContent(libPath)
	if err != nil {
		t.Fatal(err)
	}

	// A directory where the metadata file goes makes the metadata save fail
	// after the new content is copied in.
	e.manifest.Content[0].ContentHash = makeRepoFixture(t, e.fixture, "skills", "my-skill",
		map[string]string{"SKILL.md": "# v2\n", metadata.FileName + "/x": "x"})
	res, err := e.op.Install(ctx, e.request("my-skill"))
	if err != nil {
		t.Fatalf("install v2: %v", err)
	}
	if res.Items[0].Err == nil {
		t.Fatal("want the metadata save error on the item")
	}

	store, err := installstore.Load(storePath)
	if err != nil {
		t.Fatal(err)
	}
	rec := store.Find(coord)
	prevDir, _ := PreviousDirFor("example", catalog.Skills, "my-skill")
	if rec == nil || rec.Previous == nil || rec.Previous.ContentHash != v1 || rec.Previous.CopyPath != prevDir {
		t.Errorf("record = %+v, want it rotated from %s with the copy at %s", rec, v1, prevDir)
	}
}
