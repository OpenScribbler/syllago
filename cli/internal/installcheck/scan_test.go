package installcheck

import (
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
