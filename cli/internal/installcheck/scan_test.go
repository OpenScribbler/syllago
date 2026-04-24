package installcheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// seedRuleAndInstall creates a library rule, installs it into a target file
// under the tempdir, and returns the Installed view + library map ready for Scan.
func seedRuleAndInstall(t *testing.T, body []byte) (inst *installer.Installed, library map[string]*rulestore.Loaded, target string) {
	t.Helper()
	projectRoot := t.TempDir()
	homeDir := t.TempDir()

	libraryRoot := filepath.Join(projectRoot, "syllago-library")
	meta := metadata.RuleMetadata{
		ID:   "lib-id-scan",
		Name: "scan-rule",
	}
	if err := rulestore.WriteRule(libraryRoot, "claude-code", "scan-rule", meta, body); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}
	ruleDir := filepath.Join(libraryRoot, "claude-code", "scan-rule")
	loaded, err := rulestore.LoadRule(ruleDir)
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}
	library = map[string]*rulestore.Loaded{loaded.Meta.ID: loaded}

	target = filepath.Join(projectRoot, "CLAUDE.md")
	if err := installer.InstallRuleAppend(projectRoot, homeDir, "claude-code", target, "manual", loaded); err != nil {
		t.Fatalf("InstallRuleAppend: %v", err)
	}
	inst, err = installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	return inst, library, target
}

func TestScan_Clean(t *testing.T) {
	t.Parallel()
	body := []byte("# Scan me\n\nSome body.\n")
	inst, library, target := seedRuleAndInstall(t, body)

	result := Scan(inst, library)
	key := RecordKey{LibraryID: "lib-id-scan", TargetFile: target}
	got := result.PerRecord[key]
	if got.State != StateClean || got.Reason != ReasonNone {
		t.Errorf("PerRecord[%v] = %+v, want {StateClean, ReasonNone}", key, got)
	}
	targets := result.MatchSet["lib-id-scan"]
	if len(targets) != 1 || targets[0] != target {
		t.Errorf("MatchSet[lib-id-scan] = %v, want [%q]", targets, target)
	}
}

func TestScan_ModifiedEdited(t *testing.T) {
	t.Parallel()
	body := []byte("# Scan me\n\nSome body.\n")
	inst, library, target := seedRuleAndInstall(t, body)

	// Mutate target: edit a byte inside the appended block so normalization
	// can't undo the change (stripping the final \n would be canonicalized back).
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	mutated := []byte(string(raw))
	// Find "Some body" and replace one char so the pattern diverges.
	for i := 0; i < len(mutated)-1; i++ {
		if mutated[i] == 'S' && mutated[i+1] == 'o' {
			mutated[i] = 'X'
			break
		}
	}
	if err := os.WriteFile(target, mutated, 0644); err != nil {
		t.Fatalf("write mutated target: %v", err)
	}
	InvalidateCache(target) // install path normally invalidates; we bypassed that here

	result := Scan(inst, library)
	key := RecordKey{LibraryID: "lib-id-scan", TargetFile: target}
	got := result.PerRecord[key]
	if got.State != StateModified || got.Reason != ReasonEdited {
		t.Errorf("PerRecord[%v] = %+v, want {StateModified, ReasonEdited}", key, got)
	}
	if targets := result.MatchSet["lib-id-scan"]; len(targets) != 0 {
		t.Errorf("MatchSet[lib-id-scan] = %v, want empty", targets)
	}
}
