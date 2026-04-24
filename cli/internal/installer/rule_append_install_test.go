package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

func TestInstallRuleAppend_RecordsAndAppends(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	homeDir := t.TempDir()

	// Seed a library rule via rulestore.WriteRule.
	libraryRoot := filepath.Join(projectRoot, "syllago-library")
	body := []byte("# Rule body\n\nAppend me.\n")
	meta := metadata.RuleMetadata{
		ID:   "lib-id-abc",
		Name: "my-rule",
	}
	if err := rulestore.WriteRule(libraryRoot, "claude-code", "my-rule", meta, body); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}

	// Load it back so we have the full Loaded view.
	ruleDir := filepath.Join(libraryRoot, "claude-code", "my-rule")
	loaded, err := rulestore.LoadRule(ruleDir)
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}

	// Target: project-scoped CLAUDE.md.
	target := filepath.Join(projectRoot, "CLAUDE.md")

	if err := InstallRuleAppend(projectRoot, homeDir, "claude-code", target, "manual", loaded); err != nil {
		t.Fatalf("InstallRuleAppend: %v", err)
	}

	// Assert target file was appended per D20.
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	// Fresh file → "\n<canonical body>".
	canon := loaded.History[loaded.Meta.CurrentVersion]
	if canon == nil {
		t.Fatal("no history entry for CurrentVersion")
	}
	want := append([]byte{'\n'}, canon...)
	if string(got) != string(want) {
		t.Errorf("target bytes mismatch\n got %q\nwant %q", got, want)
	}

	// Assert installed.json has one matching RuleAppend entry.
	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if len(inst.RuleAppends) != 1 {
		t.Fatalf("expected 1 RuleAppend, got %d", len(inst.RuleAppends))
	}
	got0 := inst.RuleAppends[0]
	if got0.Name != loaded.Meta.Name {
		t.Errorf("Name: got %q, want %q", got0.Name, loaded.Meta.Name)
	}
	if got0.LibraryID != loaded.Meta.ID {
		t.Errorf("LibraryID: got %q, want %q", got0.LibraryID, loaded.Meta.ID)
	}
	if got0.Provider != "claude-code" {
		t.Errorf("Provider: got %q, want %q", got0.Provider, "claude-code")
	}
	if got0.TargetFile != target {
		t.Errorf("TargetFile: got %q, want %q", got0.TargetFile, target)
	}
	if got0.VersionHash != loaded.Meta.CurrentVersion {
		t.Errorf("VersionHash: got %q, want %q", got0.VersionHash, loaded.Meta.CurrentVersion)
	}
	if got0.Source != "manual" {
		t.Errorf("Source: got %q, want %q", got0.Source, "manual")
	}
	// Target is under projectRoot and not under homeDir (they're disjoint temp dirs).
	if got0.Scope != "project" {
		t.Errorf("Scope: got %q, want %q", got0.Scope, "project")
	}
	if got0.InstalledAt.IsZero() {
		t.Errorf("InstalledAt: must be set, got zero time")
	}
}
