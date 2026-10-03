package lifecycle

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// scriptedPlacer places every target except the slugs in fail, and records
// the order it was called in.
type scriptedPlacer struct {
	fail  map[string]error
	calls []string
}

func (s *scriptedPlacer) place(req InstallRequest, t Target) (installer.Placement, error) {
	s.calls = append(s.calls, t.Provider.Slug)
	notice := installer.Notice{Kind: installer.NoticeNote, Message: "placed for " + t.Provider.Slug}
	if err := s.fail[t.Provider.Slug]; err != nil {
		return installer.Placement{Notices: []installer.Notice{notice}}, err
	}
	return installer.Placement{
		Mechanism: installer.MechanismSymlink,
		Path:      filepath.Join("/dest", t.Provider.Slug, req.Item.Name),
		Notices:   []installer.Notice{notice},
	}, nil
}

var testNow = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

// isolate points HOME and the syllago directory at fresh temp dirs and
// returns the install store path.
func isolate(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	globalDir := t.TempDir()
	orig := config.GlobalDirOverride
	config.GlobalDirOverride = globalDir
	t.Cleanup(func() { config.GlobalDirOverride = orig })
	return filepath.Join(globalDir, "installs.json")
}

func scriptedModule(p placer) *Module {
	return &Module{
		placer: p,
		lock:   func() (func(), error) { return func() {}, nil },
		now:    func() time.Time { return testNow },
	}
}

func libraryItem(t *testing.T, name, registry string) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rule.md"), []byte("# "+name+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return catalog.ContentItem{Name: name, Type: catalog.Rules, Path: dir, Registry: registry}
}

func targets(slugs ...string) []Target {
	var out []Target
	for _, s := range slugs {
		out = append(out, Target{Provider: provider.Provider{Name: s, Slug: s}})
	}
	return out
}

func loadRecord(t *testing.T, storePath string, c installstore.Coord) *installstore.Record {
	t.Helper()
	s, err := installstore.Load(storePath)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	return s.Find(c)
}

func TestInstall_SecondTargetFails(t *testing.T) {
	storePath := isolate(t)
	boom := errors.New("boom")
	p := &scriptedPlacer{fail: map[string]error{"beta": boom}}
	item := libraryItem(t, "r1", "")

	out, err := scriptedModule(p).Install(InstallRequest{Item: item, Targets: targets("alpha", "beta", "gamma")})

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap the beta failure", err)
	}
	if got := p.calls; len(got) != 3 {
		t.Fatalf("placer calls = %v, want all three targets attempted", got)
	}
	if len(out.Completed) != 2 || out.Completed[0].Target.Provider.Slug != "alpha" || out.Completed[1].Target.Provider.Slug != "gamma" {
		t.Errorf("Completed = %+v, want alpha and gamma", out.Completed)
	}
	if len(out.Failed) != 1 || out.Failed[0].Stage != StagePlace || out.Failed[0].Target.Provider.Slug != "beta" {
		t.Errorf("Failed = %+v, want one place failure for beta", out.Failed)
	}
	if len(out.Notices) != 3 {
		t.Errorf("Notices = %d, want one per target including the failed one", len(out.Notices))
	}
	if !out.Changed {
		t.Error("Changed = false after two placements")
	}
	if len(out.Warnings()) != 0 {
		t.Errorf("Warnings = %v, want none for a placement failure", out.Warnings())
	}

	rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "r1"})
	if rec == nil {
		t.Fatal("no install record")
	}
	var provs []string
	for _, pl := range rec.Placements {
		provs = append(provs, pl.Provider)
	}
	if len(provs) != 2 || provs[0] != "alpha" || provs[1] != "gamma" {
		t.Errorf("recorded providers = %v, want [alpha gamma]", provs)
	}
}

func TestInstall_RecordFailureIsReported(t *testing.T) {
	isolate(t)
	item := catalog.ContentItem{Name: "gone", Type: catalog.Rules, Path: filepath.Join(t.TempDir(), "missing")}

	out, err := scriptedModule(&scriptedPlacer{}).Install(InstallRequest{Item: item, Targets: targets("alpha")})

	if err != nil {
		t.Fatalf("err = %v, want nil: the placement succeeded", err)
	}
	if len(out.Completed) != 1 {
		t.Errorf("Completed = %+v, want the placement", out.Completed)
	}
	if len(out.Failed) != 1 || out.Failed[0].Stage != StageRecord || out.Failed[0].Target.Provider.Slug != "alpha" {
		t.Fatalf("Failed = %+v, want one record failure for alpha", out.Failed)
	}
	if w := out.Warnings(); len(w) != 1 {
		t.Errorf("Warnings = %v, want the record failure", w)
	}
}

func TestInstall_Frozen(t *testing.T) {
	t.Run("registry item is pinned", func(t *testing.T) {
		storePath := isolate(t)
		item := libraryItem(t, "r2", "acme")

		out, err := scriptedModule(&scriptedPlacer{}).Install(InstallRequest{Item: item, Targets: targets("alpha"), Frozen: true})
		if err != nil || len(out.Failed) != 0 {
			t.Fatalf("err = %v, Failed = %+v", err, out.Failed)
		}
		rec := loadRecord(t, storePath, installstore.Coord{Registry: "acme", Type: "rules", Name: "r2"})
		if rec == nil || !rec.Pinned {
			t.Fatalf("record = %+v, want pinned", rec)
		}
	})
	t.Run("library item reports the pin", func(t *testing.T) {
		isolate(t)
		item := libraryItem(t, "r3", "")

		out, err := scriptedModule(&scriptedPlacer{}).Install(InstallRequest{Item: item, Targets: targets("alpha"), Frozen: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Failed) != 1 || out.Failed[0].Stage != StagePin || !errors.Is(out.Failed[0].Err, errPinNotRegistry) {
			t.Fatalf("Failed = %+v, want the only-registry pin failure", out.Failed)
		}
	})
	t.Run("library copy of a registry item is pinned", func(t *testing.T) {
		storePath := isolate(t)
		item := libraryItem(t, "r9", "")
		item.Meta = &metadata.Meta{SourceType: "registry", SourceRegistry: "acme"}

		out, err := scriptedModule(&scriptedPlacer{}).Install(InstallRequest{Item: item, Targets: targets("alpha"), Frozen: true})
		if err != nil || len(out.Failed) != 0 {
			t.Fatalf("err = %v, Failed = %+v", err, out.Failed)
		}
		rec := loadRecord(t, storePath, installstore.Coord{Registry: "acme", Type: "rules", Name: "r9"})
		if rec == nil || !rec.Pinned {
			t.Fatalf("record = %+v, want pinned under acme", rec)
		}
	})
	t.Run("nothing installed means nothing pinned", func(t *testing.T) {
		storePath := isolate(t)
		item := libraryItem(t, "r4", "acme")
		p := &scriptedPlacer{fail: map[string]error{"alpha": errors.New("boom")}}

		out, _ := scriptedModule(p).Install(InstallRequest{Item: item, Targets: targets("alpha"), Frozen: true})
		if len(out.Failed) != 1 || out.Failed[0].Stage != StagePlace {
			t.Fatalf("Failed = %+v, want only the place failure", out.Failed)
		}
		if _, err := os.Stat(storePath); !os.IsNotExist(err) {
			t.Errorf("install store exists after a failed install: %v", err)
		}
	})
}

func TestInstall_ProvenanceRecorded(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r5", "acme")
	prov := &installstore.MOATProvenance{ManifestURI: "https://example.test/m.json", SourceURI: "https://example.test/src", TrustTier: "signed"}

	if _, err := scriptedModule(&scriptedPlacer{}).Install(InstallRequest{Item: item, Targets: targets("alpha"), Provenance: prov}); err != nil {
		t.Fatal(err)
	}
	rec := loadRecord(t, storePath, installstore.Coord{Registry: "acme", Type: "rules", Name: "r5"})
	if rec == nil || rec.MOAT == nil || rec.MOAT.SourceURI != prov.SourceURI {
		t.Fatalf("record = %+v, want MOAT provenance", rec)
	}
}

// A library copy of a registry item records under its registry, with the
// commit it came from.
func TestInstall_LibraryCopyOfRegistryItem(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r8", "")
	item.Meta = &metadata.Meta{SourceType: "registry", SourceRegistry: "acme", SourceSHA: "sha-1"}

	if _, err := scriptedModule(&scriptedPlacer{}).Install(InstallRequest{Item: item, Targets: targets("alpha")}); err != nil {
		t.Fatal(err)
	}
	rec := loadRecord(t, storePath, installstore.Coord{Registry: "acme", Type: "rules", Name: "r8"})
	if rec == nil || rec.SourceSHA != "sha-1" {
		t.Fatalf("record = %+v, want one under acme with SourceSHA sha-1", rec)
	}
}

// An item's own registry wins over the registry its metadata names.
func TestInstall_ItemRegistryWinsOverMeta(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r10", "acme")
	item.Meta = &metadata.Meta{SourceType: "registry", SourceRegistry: "other"}

	if _, err := scriptedModule(&scriptedPlacer{}).Install(InstallRequest{Item: item, Targets: targets("alpha")}); err != nil {
		t.Fatal(err)
	}
	if rec := loadRecord(t, storePath, installstore.Coord{Registry: "acme", Type: "rules", Name: "r10"}); rec == nil {
		t.Fatal("no record under the item's own registry")
	}
	if rec := loadRecord(t, storePath, installstore.Coord{Registry: "other", Type: "rules", Name: "r10"}); rec != nil {
		t.Errorf("record under the metadata registry: %+v", rec)
	}
}

// seedV1 records item at its current content, then rewrites the library
// copy so the next install sees a new version. It returns the v1 hash.
func seedV1(t *testing.T, storePath string, item catalog.ContentItem) string {
	t.Helper()
	if err := installstore.RecordInstallMeta(storePath, recordCoord(item), item.Path, installstore.PlacementInput{
		Provider: "alpha", Mechanism: installstore.MechanismSymlink, Path: "/x/alpha",
	}, installstore.InstallMeta{}, testNow); err != nil {
		t.Fatal(err)
	}
	v1, err := installstore.HashContent(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(item.Path, "rule.md"), []byte("# v2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return v1
}

func TestInstall_PreviousCopyRotatesRecord(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r9", "acme")
	v1 := seedV1(t, storePath, item)

	out, err := scriptedModule(&scriptedPlacer{}).Install(InstallRequest{Item: item, Targets: targets("alpha"), PreviousCopy: "/prev/r9"})
	if err != nil || len(out.Failed) != 0 {
		t.Fatalf("err = %v, Failed = %+v", err, out.Failed)
	}
	rec := loadRecord(t, storePath, recordCoord(item))
	if rec == nil || rec.Previous == nil {
		t.Fatalf("record = %+v, want a previous version", rec)
	}
	if rec.Previous.CopyPath != "/prev/r9" || rec.Previous.ContentHash != v1 || rec.ContentHash == v1 {
		t.Errorf("Previous = %+v, ContentHash = %s; want previous %s at /prev/r9 and a new current hash", rec.Previous, rec.ContentHash, v1)
	}
}

// Rotation waits for the lock: a refused install leaves the record as it was.
func TestInstall_PreviousCopyNotRotatedWithoutLock(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r10", "acme")
	v1 := seedV1(t, storePath, item)
	m := scriptedModule(&scriptedPlacer{})
	m.lock = func() (func(), error) { return nil, errors.New("busy") }

	if _, err := m.Install(InstallRequest{Item: item, Targets: targets("alpha"), PreviousCopy: "/prev/r10"}); err == nil {
		t.Fatal("want the lock error")
	}
	rec := loadRecord(t, storePath, recordCoord(item))
	if rec == nil || rec.Previous != nil || rec.ContentHash != v1 {
		t.Errorf("record = %+v, want it unchanged at %s", rec, v1)
	}
}

func TestInstall_PreviousCopyWithoutRecord(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r11", "acme")

	out, err := scriptedModule(&scriptedPlacer{}).Install(InstallRequest{Item: item, Targets: targets("alpha"), PreviousCopy: "/prev/r11"})
	if err != nil || len(out.Failed) != 0 {
		t.Fatalf("err = %v, Failed = %+v", err, out.Failed)
	}
	if rec := loadRecord(t, storePath, recordCoord(item)); rec == nil || rec.Previous != nil {
		t.Errorf("record = %+v, want a fresh record with no previous version", rec)
	}
}

func TestInstall_LockFailureAttemptsNothing(t *testing.T) {
	isolate(t)
	p := &scriptedPlacer{}
	m := scriptedModule(p)
	busy := errors.New("busy")
	m.lock = func() (func(), error) { return nil, busy }

	out, err := m.Install(InstallRequest{Item: libraryItem(t, "r6", ""), Targets: targets("alpha", "beta")})

	if !errors.Is(err, busy) {
		t.Fatalf("err = %v, want the lock error", err)
	}
	if len(p.calls) != 0 || len(out.Unattempted) != 2 || out.Changed {
		t.Errorf("calls = %v, Unattempted = %d, Changed = %v", p.calls, len(out.Unattempted), out.Changed)
	}
}

// The tests below run the real installer on temp dirs.

func TestInstall_RealSymlink(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "sym-rule", "")
	baseDir := t.TempDir()
	prov := provider.Provider{
		Name: "Stub", Slug: "stub",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return filepath.Join(home, ".stub", "rules")
		},
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Rules },
	}

	out, err := New().Install(InstallRequest{
		Item:        item,
		ProjectRoot: t.TempDir(),
		Targets:     []Target{{Provider: prov, BaseDir: baseDir}},
		Method:      installer.MethodSymlink,
	})
	if err != nil {
		t.Fatal(err)
	}
	dest := out.Completed[0].Placement.Path
	if fi, err := os.Lstat(dest); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink: %v", dest, err)
	}
	rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "sym-rule"})
	if rec == nil || len(rec.Placements) != 1 || rec.Placements[0].Path != dest || rec.Placements[0].Mechanism != installstore.MechanismSymlink {
		t.Fatalf("record = %+v, want one symlink placement at %s", rec, dest)
	}
}

func TestInstall_RealHook(t *testing.T) {
	storePath := isolate(t)
	home := os.Getenv("HOME")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	hookDir := filepath.Join(projectRoot, "hooks", "guard")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatal(err)
	}
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"name":"guard","event":"before_tool_execute","matcher":"shell","handler":{"type":"command","command":"echo lint"}}]}`
	if err := os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "guard", Type: catalog.Hooks, Path: hookDir}

	out, err := New().Install(InstallRequest{
		Item:        item,
		ProjectRoot: projectRoot,
		Targets:     []Target{{Provider: provider.ClaudeCode}},
		Method:      installer.MethodSymlink,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Completed[0].Placement.Mechanism != installer.MechanismHookMerge {
		t.Fatalf("mechanism = %q, want hook_merge", out.Completed[0].Placement.Mechanism)
	}
	rec := loadRecord(t, storePath, installstore.Coord{Type: "hooks", Name: "guard"})
	if rec == nil || len(rec.Placements) == 0 || rec.Placements[0].Mechanism != installstore.MechanismHookMerge || rec.Placements[0].Provider != "claude-code" {
		t.Fatalf("record = %+v, want a claude-code hook_merge placement", rec)
	}
}

func TestInstall_RealRuleAppend(t *testing.T) {
	storePath := isolate(t)
	library := t.TempDir()
	meta := metadata.RuleMetadata{ID: "lib-1", Name: "append-me"}
	if err := rulestore.WriteRule(library, "claude-code", "append-me", meta, []byte("Always append.\n")); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "append-me", Type: catalog.Rules, Path: filepath.Join(library, "claude-code", "append-me")}
	projectRoot := t.TempDir()

	out, err := New().Install(InstallRequest{
		Item:        item,
		ProjectRoot: projectRoot,
		Targets:     []Target{{Provider: provider.ClaudeCode}},
		Method:      installer.MethodAppend,
		Source:      "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(projectRoot, "CLAUDE.md")
	if got := out.Completed[0].Placement.Path; got != target {
		t.Errorf("placement path = %s, want %s", got, target)
	}
	body, err := os.ReadFile(target)
	if err != nil || len(body) == 0 {
		t.Fatalf("CLAUDE.md not written: %v", err)
	}
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil || len(inst.RuleAppends) != 1 || inst.RuleAppends[0].Source != "manual" {
		t.Fatalf("installed.json rule appends = %+v, err %v", inst, err)
	}
	rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "append-me"})
	if rec == nil || len(rec.Placements) != 1 || rec.Placements[0].Mechanism != installstore.MechanismRuleAppend || rec.Placements[0].Path != target {
		t.Fatalf("record = %+v, want one rule_append placement at %s", rec, target)
	}
}

func TestInstall_RuleAppendNeedsMonolithicFile(t *testing.T) {
	isolate(t)
	item := libraryItem(t, "r7", "")
	prov := provider.Provider{Name: "No Mono", Slug: "no-mono"}

	out, err := New().Install(InstallRequest{Item: item, ProjectRoot: t.TempDir(), Targets: []Target{{Provider: prov}}, Method: installer.MethodAppend})
	if err == nil || out.Changed {
		t.Fatalf("err = %v, Changed = %v, want a refusal", err, out.Changed)
	}
}
