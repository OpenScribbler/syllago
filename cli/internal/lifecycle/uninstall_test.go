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
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// installScripted records a scripted install of item to slugs, so an
// Uninstall test starts from a record with one placement per slug.
func installScripted(t *testing.T, item catalog.ContentItem, slugs ...string) {
	t.Helper()
	if _, err := scriptedModule(&scriptedPlacer{}).Install(InstallRequest{Item: item, Targets: targets(slugs...)}); err != nil {
		t.Fatal(err)
	}
}

func TestUninstall_SecondTargetFails(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "u1", "")
	installScripted(t, item, "alpha", "beta", "gamma")
	boom := errors.New("boom")
	p := &scriptedPlacer{fail: map[string]error{"beta": boom}}

	out, err := scriptedModule(p).Uninstall(UninstallRequest{Item: item, Targets: targets("alpha", "beta", "gamma")})

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap the beta failure", err)
	}
	if len(p.calls) != 3 {
		t.Fatalf("placer calls = %v, want all three targets attempted", p.calls)
	}
	if len(out.Completed) != 2 || out.Completed[0].Target.Provider.Slug != "alpha" || out.Completed[1].Target.Provider.Slug != "gamma" {
		t.Errorf("Completed = %+v, want alpha and gamma", out.Completed)
	}
	if len(out.Failed) != 1 || out.Failed[0].Stage != StagePlace || out.Failed[0].Target.Provider.Slug != "beta" {
		t.Errorf("Failed = %+v, want one place failure for beta", out.Failed)
	}
	rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "u1"})
	if rec == nil || len(rec.Placements) != 1 || rec.Placements[0].Provider != "beta" {
		t.Fatalf("record = %+v, want only the beta placement left", rec)
	}
}

func TestUninstall_LastPlacementDropsRecord(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "u2", "")
	installScripted(t, item, "alpha")

	out, err := scriptedModule(&scriptedPlacer{}).Uninstall(UninstallRequest{Item: item, Targets: targets("alpha")})

	if err != nil || len(out.Failed) != 0 || !out.Changed {
		t.Fatalf("err = %v, Failed = %+v, Changed = %v", err, out.Failed, out.Changed)
	}
	if rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "u2"}); rec != nil {
		t.Fatalf("record = %+v, want it gone with its last placement", rec)
	}
}

func TestUninstall_RecordFailureIsReported(t *testing.T) {
	storePath := isolate(t)
	if err := os.WriteFile(storePath, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := scriptedModule(&scriptedPlacer{}).Uninstall(UninstallRequest{Item: libraryItem(t, "u3", ""), Targets: targets("alpha")})

	if err != nil {
		t.Fatalf("err = %v, want nil: the removal succeeded", err)
	}
	if len(out.Completed) != 1 {
		t.Errorf("Completed = %+v, want the removal", out.Completed)
	}
	if len(out.Failed) != 1 || out.Failed[0].Stage != StageRecord || out.Failed[0].Target.Provider.Slug != "alpha" {
		t.Fatalf("Failed = %+v, want one record failure for alpha", out.Failed)
	}
	if w := out.Warnings(); len(w) != 1 {
		t.Errorf("Warnings = %v, want the record failure", w)
	}
}

func TestUninstall_LockFailureAttemptsNothing(t *testing.T) {
	isolate(t)
	p := &scriptedPlacer{}
	m := scriptedModule(p)
	busy := errors.New("busy")
	m.lock = func() (func(), error) { return nil, busy }

	out, err := m.Uninstall(UninstallRequest{Item: libraryItem(t, "u4", ""), Targets: targets("alpha", "beta")})

	if !errors.Is(err, busy) {
		t.Fatalf("err = %v, want the lock error", err)
	}
	if len(p.calls) != 0 || len(out.Unattempted) != 2 || out.Changed {
		t.Errorf("calls = %v, Unattempted = %d, Changed = %v", p.calls, len(out.Unattempted), out.Changed)
	}
}

// The tests below run the real installer on temp dirs.

func TestUninstall_RealSymlinkUnderBaseDir(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "u5", "")
	prov := provider.Provider{
		Name: "Stub", Slug: "stub",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return filepath.Join(home, ".stub", "rules")
		},
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Rules },
	}
	target := Target{Provider: prov, BaseDir: t.TempDir()}
	projectRoot := t.TempDir()
	in, err := New().Install(InstallRequest{Item: item, ProjectRoot: projectRoot, Targets: []Target{target}, Method: installer.MethodSymlink})
	if err != nil {
		t.Fatal(err)
	}
	dest := in.Completed[0].Placement.Path

	out, err := New().Uninstall(UninstallRequest{Item: item, ProjectRoot: projectRoot, Targets: []Target{target}})

	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if got := out.Completed[0].Placement.Path; got != dest {
		t.Errorf("removed %s, want %s", got, dest)
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Errorf("%s still exists", dest)
	}
	if rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "u5"}); rec != nil {
		t.Errorf("record = %+v, want it gone", rec)
	}
}

func TestUninstall_RealRuleAppend(t *testing.T) {
	storePath := isolate(t)
	library := t.TempDir()
	meta := metadata.RuleMetadata{ID: "lib-u6", Name: "append-me"}
	if err := rulestore.WriteRule(library, "claude-code", "append-me", meta, []byte("Always append.\n")); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "append-me", Type: catalog.Rules, Path: filepath.Join(library, "claude-code", "append-me")}
	projectRoot := t.TempDir()
	targets := []Target{{Provider: provider.ClaudeCode}}
	if _, err := New().Install(InstallRequest{Item: item, ProjectRoot: projectRoot, Targets: targets, Method: installer.MethodAppend, Source: "manual"}); err != nil {
		t.Fatal(err)
	}

	out, err := New().Uninstall(UninstallRequest{Item: item, ProjectRoot: projectRoot, Targets: targets, Method: installer.MethodAppend})

	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	claudeMD := filepath.Join(projectRoot, "CLAUDE.md")
	if pl := out.Completed[0].Placement; pl.Mechanism != installer.MechanismRuleAppend || pl.Path != claudeMD {
		t.Errorf("placement = %+v, want a rule append at %s", pl, claudeMD)
	}
	body, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "Always append.") {
		t.Errorf("CLAUDE.md still holds the rule: %q", body)
	}
	if inst, err := installer.LoadInstalled(projectRoot); err != nil || len(inst.RuleAppends) != 0 {
		t.Errorf("installed.json rule appends = %+v, err %v; want none", inst, err)
	}
	if rec := loadRecord(t, storePath, installstore.Coord{Type: "rules", Name: "append-me"}); rec != nil {
		t.Errorf("record = %+v, want it gone", rec)
	}
}

func TestUninstall_RuleAppendForAnotherProvider(t *testing.T) {
	isolate(t)
	library := t.TempDir()
	meta := metadata.RuleMetadata{ID: "lib-u7", Name: "append-me"}
	if err := rulestore.WriteRule(library, "claude-code", "append-me", meta, []byte("Stay put.\n")); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "append-me", Type: catalog.Rules, Path: filepath.Join(library, "claude-code", "append-me")}
	projectRoot := t.TempDir()
	if _, err := New().Install(InstallRequest{Item: item, ProjectRoot: projectRoot, Targets: []Target{{Provider: provider.ClaudeCode}}, Method: installer.MethodAppend}); err != nil {
		t.Fatal(err)
	}

	out, err := New().Uninstall(UninstallRequest{Item: item, ProjectRoot: projectRoot, Targets: targets("other"), Method: installer.MethodAppend})

	if err == nil || out.Changed {
		t.Fatalf("err = %v, Changed = %v, want a refusal", err, out.Changed)
	}
	if inst, _ := installer.LoadInstalled(projectRoot); len(inst.RuleAppends) != 1 {
		t.Errorf("rule appends = %+v, want the claude-code record kept", inst.RuleAppends)
	}
}

// writeAppendRule writes a library rule and returns its catalog item.
func writeAppendRule(t *testing.T, library, id, body string) catalog.ContentItem {
	t.Helper()
	meta := metadata.RuleMetadata{ID: id, Name: "append-me"}
	if err := rulestore.WriteRule(library, "claude-code", "append-me", meta, []byte(body)); err != nil {
		t.Fatal(err)
	}
	return catalog.ContentItem{Name: "append-me", Type: catalog.Rules, Path: filepath.Join(library, "claude-code", "append-me")}
}

// appendTo appends item's rule to file for claude-code.
func appendTo(t *testing.T, projectRoot, file string, item catalog.ContentItem) {
	t.Helper()
	loaded, err := rulestore.LoadRule(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := installer.InstallRuleAppend(projectRoot, t.TempDir(), "claude-code", file, "manual", loaded); err != nil {
		t.Fatal(err)
	}
}

// Each target names its own file, so a file the rule cannot come out of
// does not stop the provider's other file.
func TestUninstall_RuleAppendFilesFailSeparately(t *testing.T) {
	isolate(t)
	item := writeAppendRule(t, t.TempDir(), "lib-u8", "Always append.\n")
	projectRoot := t.TempDir()
	edited := filepath.Join(projectRoot, "CLAUDE.md")
	healthy := filepath.Join(projectRoot, "sub", "CLAUDE.md")
	appendTo(t, projectRoot, edited, item)
	appendTo(t, projectRoot, healthy, item)
	if err := os.WriteFile(edited, []byte("the user rewrote this file\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cc := provider.Provider{Name: "claude-code", Slug: "claude-code"}

	out, err := New().Uninstall(UninstallRequest{
		Item:        item,
		ProjectRoot: projectRoot,
		Targets:     []Target{{Provider: cc, File: edited}, {Provider: cc, File: healthy}},
		Method:      installer.MethodAppend,
	})

	if err == nil {
		t.Fatal("err = nil, want the edited file's failure")
	}
	if len(out.Completed) != 1 || out.Completed[0].Placement.Path != healthy {
		t.Fatalf("Completed = %+v, want the healthy file", out.Completed)
	}
	if body, _ := os.ReadFile(healthy); strings.Contains(string(body), "Always append.") {
		t.Errorf("healthy file still holds the rule: %q", body)
	}
}

// Re-importing a rule gives it a new ID; its existing appends keep the old
// one and must still come out.
func TestUninstall_RuleAppendAfterReimport(t *testing.T) {
	isolate(t)
	library := t.TempDir()
	item := writeAppendRule(t, library, "lib-old", "Always append.\n")
	projectRoot := t.TempDir()
	claudeMD := filepath.Join(projectRoot, "CLAUDE.md")
	appendTo(t, projectRoot, claudeMD, item)
	writeAppendRule(t, library, "lib-new", "Always append.\n")

	_, err := New().Uninstall(UninstallRequest{
		Item:        item,
		ProjectRoot: projectRoot,
		Targets:     []Target{{Provider: provider.Provider{Name: "claude-code", Slug: "claude-code"}}},
		Method:      installer.MethodAppend,
	})

	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if inst, _ := installer.LoadInstalled(projectRoot); len(inst.RuleAppends) != 0 {
		t.Errorf("rule appends = %+v, want none", inst.RuleAppends)
	}
}
