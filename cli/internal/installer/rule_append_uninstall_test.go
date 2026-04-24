package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// seedRuleAndInstallForUninstall creates a library rule, installs it, and
// returns projectRoot, libID, target, library, and the pre-install snapshot.
func seedRuleAndInstallForUninstall(t *testing.T, preamble []byte) (projectRoot, libID, target string, library map[string]*rulestore.Loaded, preSnapshot []byte) {
	t.Helper()
	projectRoot = t.TempDir()
	homeDir := t.TempDir()

	libraryRoot := filepath.Join(projectRoot, "syllago-library")
	body := []byte("# Uninstall me\n\nSome body.\n")
	meta := metadata.RuleMetadata{ID: "lib-id-uninst", Name: "uninst-rule"}
	if err := rulestore.WriteRule(libraryRoot, "claude-code", "uninst-rule", meta, body); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}
	ruleDir := filepath.Join(libraryRoot, "claude-code", "uninst-rule")
	loaded, err := rulestore.LoadRule(ruleDir)
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}
	library = map[string]*rulestore.Loaded{loaded.Meta.ID: loaded}

	target = filepath.Join(projectRoot, "CLAUDE.md")
	if len(preamble) > 0 {
		if err := os.WriteFile(target, preamble, 0644); err != nil {
			t.Fatalf("seed preamble: %v", err)
		}
	}
	// Snapshot before install.
	if preamble != nil {
		preSnapshot = append([]byte{}, preamble...)
	} else {
		preSnapshot = nil
	}

	if err := InstallRuleAppend(projectRoot, homeDir, "claude-code", target, "manual", loaded); err != nil {
		t.Fatalf("InstallRuleAppend: %v", err)
	}
	libID = loaded.Meta.ID
	return
}

func TestUninstallRuleAppend_ExactMatch(t *testing.T) {
	t.Parallel()
	preamble := []byte("user preamble\n")
	projectRoot, libID, target, library, preSnapshot := seedRuleAndInstallForUninstall(t, preamble)

	if err := UninstallRuleAppend(projectRoot, libID, target, library); err != nil {
		t.Fatalf("UninstallRuleAppend: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != string(preSnapshot) {
		t.Errorf("post-uninstall bytes mismatch\n got %q\nwant %q", got, preSnapshot)
	}

	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if inst.FindRuleAppend(libID, target) != -1 {
		t.Errorf("installed.json still has record for (%s, %s)", libID, target)
	}
}

func TestUninstallRuleAppend_MissingTargetFileSucceeds(t *testing.T) {
	t.Parallel()
	projectRoot, libID, target, library, _ := seedRuleAndInstallForUninstall(t, []byte("P\n"))

	if err := os.Remove(target); err != nil {
		t.Fatalf("remove target: %v", err)
	}

	if err := UninstallRuleAppend(projectRoot, libID, target, library); err != nil {
		t.Fatalf("UninstallRuleAppend: %v", err)
	}

	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if inst.FindRuleAppend(libID, target) != -1 {
		t.Errorf("record still present after ENOENT uninstall")
	}
}

func TestUninstallRuleAppend_UnreadableTargetPreservesRecord(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can read 0000-mode files; skip")
	}
	projectRoot, libID, target, library, _ := seedRuleAndInstallForUninstall(t, []byte("P\n"))

	if err := os.Chmod(target, 0000); err != nil {
		t.Fatalf("chmod 0000: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0644) })

	err := UninstallRuleAppend(projectRoot, libID, target, library)
	if err == nil {
		t.Fatal("UninstallRuleAppend: expected error for unreadable target, got nil")
	}
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("error %q should contain 'reading'", err.Error())
	}

	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	if inst.FindRuleAppend(libID, target) == -1 {
		t.Errorf("record was removed despite unreadable target")
	}
}
