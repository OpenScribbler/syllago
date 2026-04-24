package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// TestReplaceRuleAppend_InPlace asserts D20's byte contract for Replace:
// splicing the recorded version's block with a new canonical body leaves the
// surrounding target bytes byte-for-byte untouched.
//
// Setup:
//  1. Seed target with "PRE\n".
//  2. Install rule A (body A1) — target becomes "PRE\n\nA1\n".
//  3. Append rule B (body B) — target becomes "PRE\n\nA1\n\nB\n".
//  4. AppendVersion(ruleA, A2) — library now has both A1 and A2 in history.
//  5. ReplaceRuleAppend(ruleA, A2) — target becomes "PRE\n\nA2\n\nB\n".
//
// The final read-back must match byte-for-byte; ruleB's block and PRE must be
// unchanged; installed.json's VersionHash for ruleA must advance to hash(A2).
func TestReplaceRuleAppend_InPlace(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	homeDir := t.TempDir()
	libraryRoot := filepath.Join(projectRoot, "syllago-library")

	// Seed library: two distinct rules. Bodies chosen so the splice boundary
	// is obvious in error output.
	bodyA1 := []byte("A1\n")
	bodyA2 := []byte("A2\n")
	bodyB := []byte("B\n")
	if err := rulestore.WriteRule(libraryRoot, "claude-code", "rule-a", metadata.RuleMetadata{ID: "lib-a", Name: "rule-a"}, bodyA1); err != nil {
		t.Fatalf("WriteRule A: %v", err)
	}
	if err := rulestore.WriteRule(libraryRoot, "claude-code", "rule-b", metadata.RuleMetadata{ID: "lib-b", Name: "rule-b"}, bodyB); err != nil {
		t.Fatalf("WriteRule B: %v", err)
	}

	ruleDirA := filepath.Join(libraryRoot, "claude-code", "rule-a")
	loadedA, err := rulestore.LoadRule(ruleDirA)
	if err != nil {
		t.Fatalf("LoadRule A: %v", err)
	}
	loadedB, err := rulestore.LoadRule(filepath.Join(libraryRoot, "claude-code", "rule-b"))
	if err != nil {
		t.Fatalf("LoadRule B: %v", err)
	}

	target := filepath.Join(projectRoot, "CLAUDE.md")
	if err := os.WriteFile(target, []byte("PRE\n"), 0644); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	// Install A then B.
	if err := InstallRuleAppend(projectRoot, homeDir, "claude-code", target, "manual", loadedA); err != nil {
		t.Fatalf("Install A: %v", err)
	}
	if err := InstallRuleAppend(projectRoot, homeDir, "claude-code", target, "manual", loadedB); err != nil {
		t.Fatalf("Install B: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read post-install: %v", err)
	}
	wantAfterInstall := "PRE\n\nA1\n\nB\n"
	if string(got) != wantAfterInstall {
		t.Fatalf("target mismatch pre-replace\n got %q\nwant %q", got, wantAfterInstall)
	}

	// Append A2 to ruleA's library history so ReplaceRuleAppend can find it.
	if err := rulestore.AppendVersion(ruleDirA, bodyA2); err != nil {
		t.Fatalf("AppendVersion A2: %v", err)
	}
	// Reload loadedA so history map includes A2.
	loadedA, err = rulestore.LoadRule(ruleDirA)
	if err != nil {
		t.Fatalf("reload A: %v", err)
	}
	library := map[string]*rulestore.Loaded{
		"lib-a": loadedA,
		"lib-b": loadedB,
	}

	if err := ReplaceRuleAppend(projectRoot, "lib-a", target, bodyA2, library); err != nil {
		t.Fatalf("ReplaceRuleAppend: %v", err)
	}

	got, err = os.ReadFile(target)
	if err != nil {
		t.Fatalf("read post-replace: %v", err)
	}
	wantAfterReplace := "PRE\n\nA2\n\nB\n"
	if string(got) != wantAfterReplace {
		t.Errorf("target mismatch post-replace\n got %q\nwant %q", got, wantAfterReplace)
	}

	// VersionHash for rule-a must have advanced to hash(A2).
	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	idx := inst.FindRuleAppend("lib-a", target)
	if idx < 0 {
		t.Fatalf("rule-a record missing after replace")
	}
	wantHash := rulestore.HashBody(bodyA2)
	if inst.RuleAppends[idx].VersionHash != wantHash {
		t.Errorf("VersionHash: got %q, want %q", inst.RuleAppends[idx].VersionHash, wantHash)
	}
}
