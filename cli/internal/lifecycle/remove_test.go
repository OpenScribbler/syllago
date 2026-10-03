package lifecycle

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func providers(slugs ...string) []provider.Provider {
	var out []provider.Provider
	for _, t := range targets(slugs...) {
		out = append(out, t.Provider)
	}
	return out
}

func confirmedRemove(item catalog.ContentItem, slugs ...string) RemoveRequest {
	return RemoveRequest{Item: item, Providers: providers(slugs...), Decisions: Decisions{RemoveConfirmed: true}}
}

func requireExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("%s: %v, want it to still exist", path, err)
	}
}

func requireGone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("%s still exists (err = %v)", path, err)
	}
}

func slugsOf(ts []Target) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Provider.Slug)
	}
	return out
}

func TestRemove_PlanChangesNothing(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r1", "")
	installScripted(t, item, "alpha", "beta")
	p := &scriptedPlacer{absent: map[string]bool{"beta": true}}

	_, err := scriptedModule(p).Remove(RemoveRequest{Item: item, Providers: providers("alpha", "beta")})

	var dr *DecisionRequired
	if !errors.As(err, &dr) || dr.Kind != RemoveConfirm {
		t.Fatalf("err = %v, want a RemoveConfirm decision", err)
	}
	plan, ok := dr.Context.(RemovePlan)
	if !ok || len(plan.Present) != 1 || plan.Present[0].Provider.Slug != "alpha" {
		t.Fatalf("Context = %+v, want a plan with alpha present", dr.Context)
	}
	if len(p.calls) != 0 {
		t.Errorf("placer calls = %v, want none", p.calls)
	}
	requireExists(t, item.Path)
	if rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "r1"}); rec == nil || len(rec.Placements) != 2 {
		t.Errorf("record = %+v, want both placements untouched", rec)
	}
}

func TestRemove_RemovesEverything(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r2", "")
	installScripted(t, item, "alpha", "beta")

	out, err := scriptedModule(&scriptedPlacer{}).Remove(confirmedRemove(item, "alpha", "beta"))

	if err != nil || len(out.Failed) != 0 || len(out.Completed) != 2 {
		t.Fatalf("err = %v, Failed = %+v, Completed = %+v", err, out.Failed, out.Completed)
	}
	requireGone(t, item.Path)
	if rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "r2"}); rec != nil {
		t.Errorf("record = %+v, want it forgotten", rec)
	}
}

// A failed target keeps the Library item and the record, and the record
// loses only the placements that were removed.
func TestRemove_SecondTargetFails(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r3", "")
	installScripted(t, item, "alpha", "beta", "gamma")
	boom := errors.New("boom")
	p := &scriptedPlacer{fail: map[string]error{"beta": boom}}

	out, err := scriptedModule(p).Remove(confirmedRemove(item, "alpha", "beta", "gamma"))

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap the beta failure", err)
	}
	if len(p.calls) != 3 {
		t.Errorf("placer calls = %v, want all three targets attempted", p.calls)
	}
	if len(out.Failed) != 1 || out.Failed[0].Target.Provider.Slug != "beta" || out.Failed[0].Stage != StagePlace {
		t.Errorf("Failed = %+v, want one place failure for beta", out.Failed)
	}
	requireExists(t, item.Path)
	rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "r3"})
	if rec == nil || len(rec.Placements) != 1 || rec.Placements[0].Provider != "beta" {
		t.Fatalf("record = %+v, want only the beta placement left", rec)
	}
}

// When every recorded placement is removed but the Remove still fails, the
// record stays, so its pin survives with the Library item.
func TestRemove_FailureKeepsEmptiedRecord(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r4", "")
	installScripted(t, item, "alpha")
	coord := installstore.Coord{Type: "rules", Name: "r4"}
	if err := installstore.SetPinned(storePath, coord, true, testNow); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	p := &scriptedPlacer{fail: map[string]error{"beta": boom}}

	if _, err := scriptedModule(p).Remove(confirmedRemove(item, "alpha", "beta")); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the beta failure", err)
	}
	rec := loadRecord(t, storePath, coord)
	if rec == nil || len(rec.Placements) != 0 || !rec.Pinned {
		t.Fatalf("record = %+v, want it kept pinned with no placements", rec)
	}
}

func TestRemove_UnknownBlocks(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r5", "")
	installScripted(t, item, "alpha", "beta", "gamma")
	unreadable := errors.New("unreadable")
	p := &scriptedPlacer{unknown: map[string]error{"beta": unreadable}}

	out, err := scriptedModule(p).Remove(confirmedRemove(item, "alpha", "beta", "gamma"))

	if !errors.Is(err, unreadable) {
		t.Fatalf("err = %v, want it to wrap the beta check failure", err)
	}
	if len(p.calls) != 0 || out.Changed {
		t.Errorf("placer calls = %v, Changed = %v; want nothing attempted", p.calls, out.Changed)
	}
	if len(out.Failed) != 1 || out.Failed[0].Stage != StagePresence || out.Failed[0].Target.Provider.Slug != "beta" {
		t.Errorf("Failed = %+v, want one presence failure for beta", out.Failed)
	}
	if got := slugsOf(out.Unattempted); len(got) != 2 || got[0] != "alpha" || got[1] != "gamma" {
		t.Errorf("Unattempted = %v, want alpha and gamma", got)
	}
	if len(out.Warnings()) != 0 {
		t.Errorf("Warnings = %v, want none: the error reports the presence failure", out.Warnings())
	}
	requireExists(t, item.Path)
	if rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "r5"}); rec == nil || len(rec.Placements) != 3 {
		t.Errorf("record = %+v, want all three placements untouched", rec)
	}
}

func TestRemove_ForgetFailsAfterDelete(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r6", "")
	installScripted(t, item, "alpha")
	storeDir := filepath.Dir(storePath)
	if err := os.Chmod(storeDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(storeDir, 0755) })

	out, err := scriptedModule(&scriptedPlacer{}).Remove(confirmedRemove(item, "alpha"))

	if err != nil {
		t.Fatalf("err = %v, want nil: the item was removed", err)
	}
	requireGone(t, item.Path)
	if len(out.Failed) != 1 || out.Failed[0].Stage != StageRecord || len(out.Warnings()) != 1 {
		t.Errorf("Failed = %+v, want one record failure shown as a warning", out.Failed)
	}
}

func TestRemove_LockFailureAttemptsNothing(t *testing.T) {
	isolate(t)
	item := libraryItem(t, "r7", "")
	p := &scriptedPlacer{}
	m := scriptedModule(p)
	locked := errors.New("locked")
	m.lock = func() (func(), error) { return nil, locked }

	out, err := m.Remove(confirmedRemove(item, "alpha", "beta"))

	if !errors.Is(err, locked) || len(p.calls) != 0 || len(out.Unattempted) != 2 {
		t.Fatalf("err = %v, calls = %v, Unattempted = %+v", err, p.calls, out.Unattempted)
	}
	requireExists(t, item.Path)
}

// A link into the item that no provider check found is in the plan, and
// Remove deletes it with the rest.
func TestRemove_StrayLink(t *testing.T) {
	isolate(t)
	home := os.Getenv("HOME")
	item := libraryItem(t, "r8", "")
	stub := provider.Provider{
		Name: "Stub", Slug: "stub",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return filepath.Join(home, ".stub", string(ct))
		},
	}
	link := filepath.Join(home, ".stub", "skills", "old-name")
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(item.Path, link); err != nil {
		t.Fatal(err)
	}
	m := scriptedModule(&scriptedPlacer{absent: map[string]bool{"stub": true}})
	req := RemoveRequest{Item: item, Providers: []provider.Provider{stub}}

	_, err := m.Remove(req)
	var dr *DecisionRequired
	if !errors.As(err, &dr) {
		t.Fatalf("err = %v, want a decision", err)
	}
	if plan := dr.Context.(RemovePlan); len(plan.Links) != 1 || plan.Links[0].Path != link {
		t.Fatalf("plan links = %+v, want %s", plan.Links, link)
	}

	req.Decisions.RemoveConfirmed = true
	out, err := m.Remove(req)
	if err != nil {
		t.Fatal(err)
	}
	requireGone(t, link)
	requireGone(t, item.Path)
	if len(out.Completed) != 1 || out.Completed[0].Placement.Path != link || out.Completed[0].Target.Provider.Name != "Stub" {
		t.Errorf("Completed = %+v, want the stub link", out.Completed)
	}
}

// An MCP uninstall that cannot write its backup fails on real disk, and the
// Library item and the record stay.
func TestRemove_RealMCPBackupIsDirectory(t *testing.T) {
	storePath := isolate(t)
	home := os.Getenv("HOME")
	projectRoot := t.TempDir()
	itemDir := filepath.Join(t.TempDir(), "srv")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{"command":"node","args":["server.js"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "srv", Type: catalog.MCP, Path: itemDir}
	if _, err := New().Install(InstallRequest{
		Item:        item,
		ProjectRoot: projectRoot,
		Targets:     []Target{{Provider: provider.ClaudeCode}},
		Method:      installer.MethodSymlink,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(home, ".claude.json.bak"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := New().Remove(RemoveRequest{
		Item:        item,
		ProjectRoot: projectRoot,
		Providers:   []provider.Provider{provider.ClaudeCode},
		Decisions:   Decisions{RemoveConfirmed: true},
	})

	if err == nil {
		t.Fatal("err = nil, want the MCP uninstall failure")
	}
	if len(out.Failed) != 1 || out.Failed[0].Stage != StagePlace {
		t.Errorf("Failed = %+v, want one place failure", out.Failed)
	}
	requireExists(t, itemDir)
	if rec := loadRecord(t, storePath, installstore.Coord{Type: "mcp", Name: "srv"}); rec == nil || len(rec.Placements) == 0 {
		t.Errorf("record = %+v, want its placement kept", rec)
	}
}

// stubProvider installs every content type under ~/.stub/<type>.
var stubProvider = provider.Provider{
	Name: "Stub", Slug: "stub",
	InstallDir: func(home string, ct catalog.ContentType) string {
		return filepath.Join(home, ".stub", string(ct))
	},
}

// A rule appended to a monolithic file comes out of that file on Remove.
func TestRemove_RealRuleAppend(t *testing.T) {
	isolate(t)
	item := writeAppendRule(t, t.TempDir(), "lib-r9", "Always append.\n")
	projectRoot := t.TempDir()
	claudeMD := filepath.Join(projectRoot, "CLAUDE.md")
	appendTo(t, projectRoot, claudeMD, item)
	req := RemoveRequest{Item: item, ProjectRoot: projectRoot, Providers: []provider.Provider{provider.ClaudeCode}}

	_, err := New().Remove(req)
	var dr *DecisionRequired
	if !errors.As(err, &dr) {
		t.Fatalf("err = %v, want a decision", err)
	}
	if plan := dr.Context.(RemovePlan); len(plan.Appends) != 1 || plan.Appends[0].File != claudeMD || plan.Appends[0].Provider.Name != provider.ClaudeCode.Name {
		t.Fatalf("plan appends = %+v, want %s for Claude Code", plan.Appends, claudeMD)
	}

	req.Decisions.RemoveConfirmed = true
	if _, err := New().Remove(req); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if body, _ := os.ReadFile(claudeMD); strings.Contains(string(body), "Always append.") {
		t.Errorf("CLAUDE.md still holds the rule: %q", body)
	}
	if inst, _ := installer.LoadInstalled(projectRoot); len(inst.RuleAppends) != 0 {
		t.Errorf("rule appends = %+v, want none", inst.RuleAppends)
	}
	requireGone(t, item.Path)
}

func TestRemove_AppendReadErrorBlocks(t *testing.T) {
	isolate(t)
	item := libraryItem(t, "r10", "")
	unreadable := errors.New("unreadable")
	p := &scriptedPlacer{appendErr: unreadable}

	out, err := scriptedModule(p).Remove(confirmedRemove(item, "alpha"))

	if !errors.Is(err, unreadable) || len(p.calls) != 0 || out.Changed {
		t.Fatalf("err = %v, calls = %v, Changed = %v; want the read error and nothing attempted", err, p.calls, out.Changed)
	}
	requireExists(t, item.Path)
}

// copyPlacer places a copy at path, outside any default install location.
type copyPlacer struct {
	scriptedPlacer
	path string
}

func (c *copyPlacer) place(InstallRequest, Target) (installer.Placement, error) {
	return installer.Placement{Mechanism: installer.MechanismCopy, Path: c.path}, nil
}

// An install the provider checks cannot see, such as one under a custom
// base directory, keeps the Library item rather than being orphaned.
func TestRemove_UnreachedPlacementKeepsItem(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r11", "")
	copied := filepath.Join(t.TempDir(), "custom", "r11")
	if err := os.MkdirAll(copied, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := scriptedModule(&copyPlacer{path: copied}).Install(InstallRequest{Item: item, Targets: targets("alpha")}); err != nil {
		t.Fatal(err)
	}

	out, err := scriptedModule(&scriptedPlacer{absent: map[string]bool{"alpha": true}}).Remove(confirmedRemove(item, "alpha"))

	if err == nil || !strings.Contains(err.Error(), copied) {
		t.Fatalf("err = %v, want it to name %s", err, copied)
	}
	if len(out.Failed) != 1 || out.Failed[0].Stage != StagePlace || out.Failed[0].Target.Provider.Slug != "alpha" {
		t.Errorf("Failed = %+v, want one place failure for alpha", out.Failed)
	}
	requireExists(t, item.Path)
	if rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "r11"}); rec == nil || len(rec.Placements) != 1 {
		t.Errorf("record = %+v, want the copy placement kept", rec)
	}
}

// A provider directory that cannot be read may hide a link into the item.
func TestRemove_LinkScanErrorBlocks(t *testing.T) {
	isolate(t)
	item := libraryItem(t, "r12", "")
	sealed := filepath.Join(os.Getenv("HOME"), ".stub", "skills")
	if err := os.MkdirAll(sealed, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sealed, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(sealed, 0755) })
	p := &scriptedPlacer{absent: map[string]bool{"stub": true}}

	out, err := scriptedModule(p).Remove(RemoveRequest{Item: item, Providers: []provider.Provider{stubProvider}, Decisions: Decisions{RemoveConfirmed: true}})

	if err == nil || len(p.calls) != 0 || out.Changed {
		t.Fatalf("err = %v, calls = %v, Changed = %v; want a refusal before any change", err, p.calls, out.Changed)
	}
	if len(out.Failed) != 1 || out.Failed[0].Stage != StagePresence || out.Failed[0].Target.Provider.Name != "Stub" {
		t.Errorf("Failed = %+v, want one presence failure for Stub", out.Failed)
	}
	requireExists(t, item.Path)
}

// Without the install records Remove cannot tell what it would orphan.
func TestRemove_CorruptStoreBlocks(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r13", "")
	if err := os.WriteFile(storePath, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	p := &scriptedPlacer{}

	out, err := scriptedModule(p).Remove(confirmedRemove(item, "alpha"))

	if err == nil || len(p.calls) != 0 || out.Changed {
		t.Fatalf("err = %v, calls = %v, Changed = %v; want a refusal before any change", err, p.calls, out.Changed)
	}
	if len(out.Failed) != 1 || out.Failed[0].Stage != StagePresence || len(out.Warnings()) != 0 {
		t.Errorf("Failed = %+v, want one presence failure and no warnings", out.Failed)
	}
	requireExists(t, item.Path)
}

// A Library item that cannot be deleted keeps its record, emptied of the
// placements Remove took out.
func TestRemove_LibraryDeleteFails(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "r14", "")
	installScripted(t, item, "alpha")
	if err := os.Chmod(item.Path, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(item.Path, 0755) })

	_, err := scriptedModule(&scriptedPlacer{}).Remove(confirmedRemove(item, "alpha"))

	if err == nil {
		t.Fatal("err = nil, want the library delete failure")
	}
	requireExists(t, item.Path)
	if rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "r14"}); rec == nil || len(rec.Placements) != 0 {
		t.Errorf("record = %+v, want it kept with no placements", rec)
	}
}

func TestRemove_StrayLinkRemovalFails(t *testing.T) {
	isolate(t)
	item := libraryItem(t, "r15", "")
	dir := filepath.Join(os.Getenv("HOME"), ".stub", "skills")
	link := filepath.Join(dir, "old-name")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(item.Path, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })
	p := &scriptedPlacer{absent: map[string]bool{"stub": true}}

	out, err := scriptedModule(p).Remove(RemoveRequest{Item: item, Providers: []provider.Provider{stubProvider}, Decisions: Decisions{RemoveConfirmed: true}})

	if err == nil {
		t.Fatal("err = nil, want the link removal failure")
	}
	if len(out.Failed) != 1 || out.Failed[0].Stage != StagePlace {
		t.Errorf("Failed = %+v, want one place failure", out.Failed)
	}
	requireExists(t, link)
	requireExists(t, item.Path)
}

// dirProvider installs item types it supports as symlinks under
// ~/<dir>/<type>, or under ~/<dir>/shared for the types in shared.
func dirProvider(slug, dir string, legacy string, shared ...catalog.ContentType) provider.Provider {
	p := provider.Provider{
		Name: slug, Slug: slug,
		InstallDir: func(home string, ct catalog.ContentType) string {
			for _, s := range shared {
				if ct == s {
					return filepath.Join(home, dir, "shared")
				}
			}
			return filepath.Join(home, dir, string(ct))
		},
		SupportsType: func(catalog.ContentType) bool { return true },
	}
	if legacy != "" {
		p.LegacyInstallDir = func(home string, ct catalog.ContentType) string {
			return filepath.Join(home, legacy, string(ct))
		}
	}
	return p
}

func installSymlink(t *testing.T, item catalog.ContentItem, projectRoot string, prov provider.Provider) string {
	t.Helper()
	out, err := New().Install(InstallRequest{
		Item:        item,
		ProjectRoot: projectRoot,
		Targets:     []Target{{Provider: prov}},
		Method:      installer.MethodSymlink,
	})
	if err != nil {
		t.Fatal(err)
	}
	return out.Completed[0].Placement.Path
}

func confirmedReal(item catalog.ContentItem, projectRoot string, provs ...provider.Provider) RemoveRequest {
	return RemoveRequest{Item: item, ProjectRoot: projectRoot, Providers: provs, Decisions: Decisions{RemoveConfirmed: true}}
}

// Two providers that install to the same directory share one install, and
// removing it for the first removes it for both.
func TestRemove_SharedInstallDir(t *testing.T) {
	isolate(t)
	item := libraryItem(t, "r16", "")
	first := dirProvider("first", ".shared", "")
	second := dirProvider("second", ".shared", "")
	dest := installSymlink(t, item, t.TempDir(), first)

	out, err := New().Remove(confirmedReal(item, t.TempDir(), first, second))

	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	requireGone(t, dest)
	requireGone(t, item.Path)
	if got := slugsOf(targetsOf(out.Completed)); len(got) != 2 {
		t.Errorf("Completed = %v, want both providers", got)
	}
}

// A provider that installs two types to one directory has its install
// removed once, whatever type the link scan files the link under.
func TestRemove_SharedTypeDir(t *testing.T) {
	isolate(t)
	item := libraryItem(t, "r17", "")
	item.Type = catalog.Commands
	prov := dirProvider("mixed", ".mixed", "", catalog.Rules, catalog.Commands)
	dest := installSymlink(t, item, t.TempDir(), prov)

	if _, err := New().Remove(confirmedReal(item, t.TempDir(), prov)); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	requireGone(t, dest)
	requireGone(t, item.Path)
}

// A link of the same name in the legacy directory is not the install
// Remove uninstalls, so Remove deletes it as a stray link.
func TestRemove_LegacyLinkBesideInstall(t *testing.T) {
	isolate(t)
	item := libraryItem(t, "r18", "")
	prov := dirProvider("moved", ".moved", ".moved-old")
	dest := installSymlink(t, item, t.TempDir(), prov)
	legacy := filepath.Join(os.Getenv("HOME"), ".moved-old", "rules", filepath.Base(dest))
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(item.Path, legacy); err != nil {
		t.Fatal(err)
	}

	if _, err := New().Remove(confirmedReal(item, t.TempDir(), prov)); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	requireGone(t, dest)
	requireGone(t, legacy)
	requireGone(t, item.Path)
}

// An MCP server merged into another project's config is an install Remove
// does not reach, so the Library item and the record stay.
func TestRemove_MCPInAnotherProjectKeepsItem(t *testing.T) {
	storePath := isolate(t)
	itemDir := filepath.Join(t.TempDir(), "srv2")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{"command":"node","args":["server.js"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "srv2", Type: catalog.MCP, Path: itemDir}
	if _, err := New().Install(InstallRequest{
		Item:        item,
		ProjectRoot: t.TempDir(),
		Targets:     []Target{{Provider: provider.Cursor}},
		Method:      installer.MethodSymlink,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := New().Remove(confirmedReal(item, t.TempDir(), provider.Cursor)); err == nil {
		t.Fatal("err = nil, want the unreached MCP install reported")
	}
	requireExists(t, itemDir)
	if rec := loadRecord(t, storePath, installstore.Coord{Type: "mcp", Name: "srv2"}); rec == nil || len(rec.Placements) == 0 {
		t.Errorf("record = %+v, want its placement kept", rec)
	}
}

// A rule appended to another project's file is an install Remove does not
// reach, so the Library item stays.
func TestRemove_AppendInAnotherProjectKeepsItem(t *testing.T) {
	isolate(t)
	item := writeAppendRule(t, t.TempDir(), "lib-r19", "Elsewhere.\n")
	if _, err := New().Install(InstallRequest{
		Item:        item,
		ProjectRoot: t.TempDir(),
		Targets:     []Target{{Provider: provider.ClaudeCode}},
		Method:      installer.MethodAppend,
		Source:      "manual",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := New().Remove(confirmedReal(item, t.TempDir(), provider.ClaudeCode)); err == nil {
		t.Fatal("err = nil, want the unreached append reported")
	}
	requireExists(t, item.Path)
}

func targetsOf(steps []Step) []Target {
	var out []Target
	for _, s := range steps {
		out = append(out, s.Target)
	}
	return out
}
