package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// TestApp_RescanComputesVerification verifies the D16 rescan hook: given a
// project with a library rule installed into a target, computeVerification
// returns a VerificationResult with a Clean MatchSet entry. Modifying the
// target file then recomputing transitions the entry to Modified — so when
// the rescan result is applied to App state via handleCatalogReady, the
// library's Installed column reflects the new on-disk truth.
func TestApp_RescanComputesVerification(t *testing.T) {
	t.Parallel()

	// Build a tiny library rule via rulestore.WriteRule + InstallRuleAppend
	// into a project target. After install: MatchSet[libID] should contain
	// targetFile. After mutating targetFile: the entry should drop.
	contentRoot := t.TempDir()
	projectRoot := t.TempDir()
	homeDir := t.TempDir()
	rulesRoot := filepath.Join(contentRoot, "rules")

	body := []byte("# Test rule\n\nHelpful text.\n")
	meta := metadata.RuleMetadata{
		ID:          "lib-rescan-test",
		Name:        "rescan-test",
		Description: "A rule used to verify the TUI rescan hook",
	}
	if err := rulestore.WriteRule(rulesRoot, "manual", "rescan-test", meta, body); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}
	ruleDir := filepath.Join(rulesRoot, "manual", "rescan-test")
	loaded, err := rulestore.LoadRule(ruleDir)
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}

	target := filepath.Join(projectRoot, "CLAUDE.md")
	if err := installer.InstallRuleAppend(projectRoot, homeDir, "claude-code", target, "manual", loaded); err != nil {
		t.Fatalf("InstallRuleAppend: %v", err)
	}

	item := catalog.ContentItem{
		Name:    "rescan-test",
		Type:    catalog.Rules,
		Path:    ruleDir,
		Library: true,
		Source:  "library",
		Meta:    &metadata.Meta{ID: "lib-rescan-test"},
	}

	// Phase 1: fresh install — rescan should see Clean.
	result := computeVerification(projectRoot, []catalog.ContentItem{item})
	if result == nil {
		t.Fatalf("computeVerification returned nil")
	}
	if targets := result.MatchSet["lib-rescan-test"]; len(targets) != 1 || targets[0] != target {
		t.Fatalf("after install expected MatchSet[lib-rescan-test] = [%s], got %v", target, result.MatchSet["lib-rescan-test"])
	}

	// Phase 2: mutate the target file (simulate external edit) and rescan.
	// The block is no longer byte-equal to the recorded history body, so
	// the State transitions to Modified and MatchSet drops the entry.
	if err := os.WriteFile(target, []byte("# Completely different contents\n"), 0644); err != nil {
		t.Fatalf("mutate target: %v", err)
	}
	// Bump mtime so the mtime cache doesn't reuse the Clean result.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(target, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	result2 := computeVerification(projectRoot, []catalog.ContentItem{item})
	if result2 == nil {
		t.Fatalf("computeVerification returned nil on rescan")
	}
	if targets := result2.MatchSet["lib-rescan-test"]; len(targets) != 0 {
		t.Fatalf("after mutation expected MatchSet[lib-rescan-test] empty, got %v", targets)
	}
}
