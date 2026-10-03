package tui

import (
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

// skillProvider installs skills under <home>/.<slug>/skills.
func skillProvider(slug string) provider.Provider {
	return provider.Provider{
		Name: slug,
		Slug: slug,
		InstallDir: func(home string, ct catalog.ContentType) string {
			return filepath.Join(home, "."+slug, "skills")
		},
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
	}
}

func registrySkill(t *testing.T) catalog.ContentItem {
	t.Helper()
	libraryPath := filepath.Join(t.TempDir(), "skills", "writer")
	writeTUITestFile(t, filepath.Join(libraryPath, "SKILL.md"), []byte("# Writer\n"))
	return catalog.ContentItem{
		Name: "writer",
		Type: catalog.Skills,
		Path: libraryPath,
		Meta: &metadata.Meta{
			SourceType:     "registry",
			SourceRegistry: "acme/tools",
			SourceSHA:      "sha-from-meta",
		},
	}
}

var writerCoord = installstore.Coord{Registry: "acme/tools", Type: string(catalog.Skills), Name: "writer"}

func TestDoInstallCmd_RecordsInstall(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	app := testApp(t)
	baseDir := t.TempDir()

	msg := app.doInstallCmd(installResultMsg{
		item:     registrySkill(t),
		provider: skillProvider("test-prov"),
		method:   installer.MethodSymlink,
		location: baseDir,
	})().(installDoneMsg)
	if msg.err != nil || len(msg.warnings) != 0 {
		t.Fatalf("err = %v, warnings = %q", msg.err, msg.warnings)
	}

	rec := mustLoadTUIInstallRecordStore(t, configDir).Find(writerCoord)
	if rec == nil {
		t.Fatal("install record missing")
	}
	if rec.SourceSHA != "sha-from-meta" {
		t.Errorf("SourceSHA = %q, want sha-from-meta", rec.SourceSHA)
	}
	want := filepath.Join(baseDir, ".test-prov", "skills", "writer")
	if len(rec.Placements) != 1 || rec.Placements[0].Path != want {
		t.Errorf("placements = %+v, want one at %s", rec.Placements, want)
	}
}

// The TUI used to drop record errors, so an install looked clean while the
// store never heard of it.
func TestDoInstallCmd_RecordFailureWarns(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	if err := os.MkdirAll(filepath.Join(configDir, "installs.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	app := testApp(t)

	msg := app.doInstallCmd(installResultMsg{
		item:     registrySkill(t),
		provider: skillProvider("test-prov"),
		method:   installer.MethodSymlink,
		location: t.TempDir(),
	})().(installDoneMsg)
	if msg.err != nil {
		t.Fatalf("install should succeed, got %v", msg.err)
	}
	if len(msg.warnings) != 1 || !strings.Contains(msg.warnings[0], "could not record install state") {
		t.Fatalf("warnings = %q, want one record warning", msg.warnings)
	}
}

func TestDoInstallAllCmd_CountsAndFirstErr(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	t.Setenv("HOME", t.TempDir())
	// A file where the second provider's skills directory should be makes
	// its symlink fail.
	home, _ := os.UserHomeDir()
	writeTUITestFile(t, filepath.Join(home, ".broken"), []byte("not a dir"))
	app := testApp(t)

	msg := app.doInstallAllCmd(installAllResultMsg{
		item:      registrySkill(t),
		providers: []provider.Provider{skillProvider("good"), skillProvider("broken")},
	})().(installAllDoneMsg)
	if msg.count != 1 {
		t.Errorf("count = %d, want 1", msg.count)
	}
	if msg.firstErr == nil {
		t.Error("firstErr = nil, want the broken provider's error")
	}
	rec := mustLoadTUIInstallRecordStore(t, configDir).Find(writerCoord)
	if rec == nil || len(rec.Placements) != 1 || rec.Placements[0].Provider != "good" {
		t.Fatalf("record = %+v, want one placement for good", rec)
	}
}

func TestDoInstallAppendCmd_RecordsRuleAppend(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	t.Setenv("HOME", t.TempDir())
	library := t.TempDir()
	meta := metadata.RuleMetadata{ID: "lib-1", Name: "append-me"}
	if err := rulestore.WriteRule(library, "claude-code", "append-me", meta, []byte("Always append.\n")); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "append-me", Type: catalog.Rules, Path: filepath.Join(library, "claude-code", "append-me")}
	projectRoot := t.TempDir()
	app := testApp(t)

	msg := app.doInstallCmd(installResultMsg{
		item:        item,
		provider:    provider.ClaudeCode,
		method:      installer.MethodAppend,
		projectRoot: projectRoot,
	})().(installDoneMsg)
	target := filepath.Join(projectRoot, "CLAUDE.md")
	if msg.err != nil || msg.targetPath != target {
		t.Fatalf("err = %v, targetPath = %q, want %s", msg.err, msg.targetPath, target)
	}
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil || len(inst.RuleAppends) != 1 || inst.RuleAppends[0].Source != "tui" {
		t.Fatalf("installed.json rule appends = %+v, err %v", inst, err)
	}
	rec := mustLoadTUIInstallRecordStore(t, configDir).Find(installstore.Coord{Type: "rules", Name: "append-me"})
	if rec == nil || len(rec.Placements) != 1 || rec.Placements[0].Mechanism != installstore.MechanismRuleAppend {
		t.Fatalf("record = %+v, want one rule_append placement", rec)
	}
}

func TestActions_HandleInstallDone_WarningsShow(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.handleInstallDone(installDoneMsg{
		itemName:     "my-skill",
		providerName: "Claude Code",
		warnings:     []string{"could not record install state: disk full"},
	})
	result := m.(App)
	cur := result.toast.Current()
	if cur == nil || cur.level != toastWarning {
		t.Fatalf("install with warnings should raise a warning toast, got %+v", cur)
	}
	if len(cur.details) != 1 || cur.details[0] != "warning: could not record install state: disk full" {
		t.Errorf("details = %q", cur.details)
	}
}

func TestActions_HandleInstallAllDone_WarningsShow(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.handleInstallAllDone(installAllDoneMsg{
		itemName: "my-skill",
		count:    2,
		warnings: []string{"could not record install state: disk full"},
	})
	result := m.(App)
	cur := result.toast.Current()
	if cur == nil || cur.level != toastWarning || len(cur.details) != 1 {
		t.Fatalf("install-all with warnings should raise a warning toast with details, got %+v", cur)
	}
}
