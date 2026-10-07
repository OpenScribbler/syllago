package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// TestLoadStartupSnapshot_VerifiesAfterCleanup installs a library rule that
// was also promoted to shared content. Startup cleanup removes the library
// copy, so the rule must not show as installed: verification has to run
// after the cleanup, or its cache keeps the pre-cleanup Clean state.
func TestLoadStartupSnapshot_VerifiesAfterCleanup(t *testing.T) {
	isolateListEnv(t)
	globalDir := t.TempDir()
	withGlobalLibrary(t, globalDir)
	root := t.TempDir()
	homeDir := t.TempDir()

	meta := metadata.RuleMetadata{ID: "lib-promoted", Name: "promoted"}
	body := []byte("# Promoted rule\n\nHelpful text.\n")
	libRules := filepath.Join(globalDir, "rules")
	for _, dir := range []string{libRules, filepath.Join(root, "rules")} {
		if err := rulestore.WriteRule(dir, "claude-code", "promoted", meta, body); err != nil {
			t.Fatalf("WriteRule: %v", err)
		}
	}
	libRule := filepath.Join(libRules, "claude-code", "promoted")
	loaded, err := rulestore.LoadRule(libRule)
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}
	target := filepath.Join(root, "CLAUDE.md")
	if err := installer.InstallRuleAppend(root, homeDir, "claude-code", target, "manual", loaded); err != nil {
		t.Fatalf("InstallRuleAppend: %v", err)
	}

	snap, cleaned, err := loadStartupSnapshot(root, root)
	if err != nil {
		t.Fatalf("loadStartupSnapshot: %v", err)
	}
	if len(cleaned) != 1 {
		t.Fatalf("cleaned: want the 1 promoted library rule, got %+v", cleaned)
	}
	if _, err := os.Stat(libRule); !os.IsNotExist(err) {
		t.Fatalf("library copy still present after cleanup: %v", err)
	}
	if got := snap.Verification.MatchSet["lib-promoted"]; len(got) != 0 {
		t.Errorf("MatchSet[lib-promoted]: want empty once the library copy is gone, got %v", got)
	}
}

// TestLoadStartupSnapshot_ScanError checks that a content root the scan
// cannot read fails the load.
func TestLoadStartupSnapshot_ScanError(t *testing.T) {
	isolateListEnv(t)
	withGlobalLibrary(t, t.TempDir())
	missing := filepath.Join(t.TempDir(), "missing")

	snap, _, err := loadStartupSnapshot(missing, t.TempDir())
	if err == nil || snap != nil {
		t.Fatalf("want (nil, error) for a missing content root, got (%v, %v)", snap, err)
	}
}
