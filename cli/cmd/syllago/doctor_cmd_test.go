package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestDoctorCheckLibrary(t *testing.T) {
	// With a valid library dir
	dir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	os.MkdirAll(filepath.Join(dir, "skills", "my-skill"), 0755)

	c := checkLibrary()
	if c.Status != checkOK {
		t.Errorf("expected ok, got %s: %s", c.Status, c.Message)
	}
	if !strings.Contains(c.Message, "1 items") {
		t.Errorf("expected item count in message, got: %s", c.Message)
	}
}

func TestDoctorCheckLibraryMissing(t *testing.T) {
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = filepath.Join(t.TempDir(), "nonexistent")
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	c := checkLibrary()
	if c.Status != checkErr {
		t.Errorf("expected err for missing library, got %s", c.Status)
	}
}

func TestDoctorCheckProviders(t *testing.T) {
	c := checkProviders()
	// On any dev machine, at least some providers should be detected
	if c.Status == checkErr {
		t.Errorf("unexpected error status: %s", c.Message)
	}
	if !strings.Contains(c.Message, "detected") {
		t.Errorf("expected 'detected' in message, got: %s", c.Message)
	}
}

func TestDoctorCheckSymlinksEmpty(t *testing.T) {
	dir := t.TempDir()
	c := checkSymlinks(dir)
	if c.Status != checkOK {
		t.Errorf("expected ok for empty installed.json, got %s", c.Status)
	}
}

func TestDoctorCheckContentDriftClean(t *testing.T) {
	dir := t.TempDir()
	c := checkContentDrift(dir)
	if c.Status != checkOK {
		t.Errorf("expected ok for no installed content, got %s: %s", c.Status, c.Message)
	}
}

func TestCheckContentDrift_MissingInstalledJSON(t *testing.T) {
	// A fresh temp dir has no .syllago/installed.json at all.
	// LoadInstalled treats this as empty (not an error), so VerifyIntegrity
	// returns zero drift entries and checkContentDrift reports OK.
	dir := t.TempDir()
	c := checkContentDrift(dir)
	if c.Status != checkOK {
		t.Errorf("Status = %s, want ok when installed.json is absent", c.Status)
	}
	if c.Name != "integrity" {
		t.Errorf("Name = %q, want %q", c.Name, "integrity")
	}
	if !strings.Contains(c.Message, "no content drift") {
		t.Errorf("Message = %q, want it to mention no drift", c.Message)
	}
}

func TestCheckContentDrift_CleanWithTrackedContent(t *testing.T) {
	// Create a project root with installed.json tracking a symlink whose
	// target file exists and matches the recorded hash — no drift.
	dir := t.TempDir()

	// Create the target file that the symlink would point to.
	targetFile := filepath.Join(dir, "library", "skills", "greeting", "SKILL.md")
	os.MkdirAll(filepath.Dir(targetFile), 0755)
	content := []byte("# Greeting Skill\nSays hello.\n")
	os.WriteFile(targetFile, content, 0644)

	// Compute the hash that VerifyIntegrity will compare against.
	hash := installer.HashBytes(content)

	// Write installed.json with a symlink entry whose hash matches.
	inst := &installer.Installed{
		Symlinks: []installer.InstalledSymlink{
			{
				Path:        filepath.Join(dir, ".claude", "rules", "greeting"),
				Target:      targetFile,
				ContentHash: hash,
				Source:      "test",
				InstalledAt: time.Now(),
			},
		},
	}
	if err := installer.SaveInstalled(dir, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	c := checkContentDrift(dir)
	if c.Status != checkOK {
		t.Errorf("Status = %s, want ok when hashes match; message: %s", c.Status, c.Message)
	}
	if len(c.Details) != 0 {
		t.Errorf("Details = %v, want empty for clean state", c.Details)
	}
}

func TestCheckContentDrift_DetectsDrift(t *testing.T) {
	// Track a file with one hash, then change the file content so the
	// hash no longer matches — checkContentDrift should report warn.
	dir := t.TempDir()

	targetFile := filepath.Join(dir, "library", "skills", "greeting", "SKILL.md")
	os.MkdirAll(filepath.Dir(targetFile), 0755)

	originalContent := []byte("# Original content")
	os.WriteFile(targetFile, originalContent, 0644)
	originalHash := installer.HashBytes(originalContent)

	// Write installed.json with the original hash.
	inst := &installer.Installed{
		Symlinks: []installer.InstalledSymlink{
			{
				Path:        filepath.Join(dir, ".claude", "rules", "greeting"),
				Target:      targetFile,
				ContentHash: originalHash,
				Source:      "test",
				InstalledAt: time.Now(),
			},
		},
	}
	if err := installer.SaveInstalled(dir, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	// Now modify the target file so it drifts.
	os.WriteFile(targetFile, []byte("# Modified content"), 0644)

	c := checkContentDrift(dir)
	if c.Status != checkWarn {
		t.Errorf("Status = %s, want warn for drifted content", c.Status)
	}
	if !strings.Contains(c.Message, "1 item(s) modified") {
		t.Errorf("Message = %q, want mention of 1 modified item", c.Message)
	}
	if len(c.Details) != 1 {
		t.Errorf("Details len = %d, want 1", len(c.Details))
	} else if !strings.Contains(c.Details[0], "modified") {
		t.Errorf("Details[0] = %q, want it to contain 'modified'", c.Details[0])
	}
}

func TestCheckContentDrift_MissingTarget(t *testing.T) {
	// Track a symlink whose target file no longer exists on disk.
	// VerifyIntegrity reports this as "missing" drift.
	dir := t.TempDir()

	inst := &installer.Installed{
		Symlinks: []installer.InstalledSymlink{
			{
				Path:        filepath.Join(dir, ".claude", "rules", "gone"),
				Target:      filepath.Join(dir, "nonexistent", "file.md"),
				ContentHash: "abc123fakehash",
				Source:      "test",
				InstalledAt: time.Now(),
			},
		},
	}
	if err := installer.SaveInstalled(dir, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	c := checkContentDrift(dir)
	if c.Status != checkWarn {
		t.Errorf("Status = %s, want warn for missing target", c.Status)
	}
	if len(c.Details) != 1 {
		t.Errorf("Details len = %d, want 1", len(c.Details))
	} else if !strings.Contains(c.Details[0], "missing") {
		t.Errorf("Details[0] = %q, want it to contain 'missing'", c.Details[0])
	}
}

func TestCheckContentDrift_CorruptInstalledJSON(t *testing.T) {
	// If installed.json contains invalid JSON, VerifyIntegrity returns an
	// error and checkContentDrift should report warn with the error detail.
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".syllago"), 0755)
	os.WriteFile(filepath.Join(dir, ".syllago", "installed.json"), []byte("{invalid"), 0644)

	c := checkContentDrift(dir)
	if c.Status != checkWarn {
		t.Errorf("Status = %s, want warn for corrupt installed.json", c.Status)
	}
	if !strings.Contains(c.Message, "could not verify") {
		t.Errorf("Message = %q, want 'could not verify'", c.Message)
	}
	if len(c.Details) == 0 {
		t.Error("Details should contain the error message")
	}
}

func TestCheckContentDrift_SkipsEntriesWithoutHash(t *testing.T) {
	// Symlinks installed before hash tracking was added have an empty
	// ContentHash. VerifyIntegrity skips these — they shouldn't be
	// reported as drift.
	dir := t.TempDir()

	inst := &installer.Installed{
		Symlinks: []installer.InstalledSymlink{
			{
				Path:        filepath.Join(dir, ".claude", "rules", "legacy"),
				Target:      filepath.Join(dir, "whatever"),
				ContentHash: "", // no hash recorded
				Source:      "test",
				InstalledAt: time.Now(),
			},
		},
	}
	if err := installer.SaveInstalled(dir, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	c := checkContentDrift(dir)
	if c.Status != checkOK {
		t.Errorf("Status = %s, want ok when entries have no hash", c.Status)
	}
}

func TestDoctorCheckRegistriesNone(t *testing.T) {
	c := checkRegistriesWith("")
	if c.Status != checkOK {
		t.Errorf("expected ok with no registries, got %s", c.Status)
	}
}

func TestDoctorJSONOutput(t *testing.T) {
	dir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return dir, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	// We can't test the full command because os.Exit would kill the test,
	// but we can test the check functions produce valid JSON-serializable results.
	checks := []checkResult{
		checkLibrary(),
		checkProviders(),
		checkSymlinks(dir),
		checkContentDrift(dir),
		checkRegistriesWith(""),
	}

	result := doctorResult{Checks: checks, Summary: "test"}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal doctor result: %v", err)
	}

	// Verify it round-trips
	var parsed doctorResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal doctor result: %v", err)
	}
	if len(parsed.Checks) != len(checks) {
		t.Errorf("expected %d checks, got %d", len(checks), len(parsed.Checks))
	}

	_ = stdout // output captured but not used for this test
}

func TestDoctorCmdRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "doctor" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected doctor command registered on rootCmd")
	}
}

// captureOsExit swaps the osExit seam for a test that records the code.
// Returns a pointer to the captured code (0 = not called) and registers cleanup.
func captureOsExit(t *testing.T) *int {
	t.Helper()
	var captured int
	orig := osExit
	osExit = func(code int) { captured = code }
	t.Cleanup(func() { osExit = orig })
	return &captured
}

func TestRunDoctor_MissingLibraryExitsWith2(t *testing.T) {
	// Library missing → checkLibrary returns checkErr → runDoctor calls osExit(2).
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = filepath.Join(t.TempDir(), "nonexistent")
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return "", nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	output.SetForTest(t)
	exitCode := captureOsExit(t)

	err := runDoctor(doctorCmd, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if *exitCode != 2 {
		t.Errorf("expected osExit(2), got osExit(%d)", *exitCode)
	}
}

func TestRunDoctor_ReturnsNilWhenAllClean(t *testing.T) {
	// With a valid library and project root, runDoctor should return without error.
	// The exact osExit behavior depends on the environment (providers detected,
	// registries configured), so we only assert that RunE returns nil.
	dir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return dir, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	output.SetForTest(t)
	captureOsExit(t)

	err := runDoctor(doctorCmd, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestRunDoctor_JSONOutput(t *testing.T) {
	// JSON output mode emits the doctor result as JSON on stdout.
	dir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return dir, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	captureOsExit(t)

	err := runDoctor(doctorCmd, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var result doctorResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, stdout.String())
	}
	if len(result.Checks) == 0 {
		t.Error("expected at least one check in JSON output")
	}
	if result.Summary == "" {
		t.Error("expected summary in JSON output")
	}
}

func TestCheckNamingQuality_MissingDisplayNames(t *testing.T) {
	// A hook item without a DisplayName triggers the unnamed count.
	dir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	hookDir := filepath.Join(dir, "hooks", "my-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hook.yaml"), []byte("events: []\n"), 0644)

	c := checkNamingQuality(dir)
	if c.Status != checkWarn {
		t.Errorf("Status = %s, want warn when hooks lack display names; message: %s", c.Status, c.Message)
	}
	if !strings.Contains(c.Message, "hooks/MCP items have no display name") {
		t.Errorf("Message = %q, want mention of missing display names", c.Message)
	}
}

func TestCheckNamingQuality_AllNamed(t *testing.T) {
	// No hooks or MCP items → 0 unnamed → checkOK.
	dir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	c := checkNamingQuality(dir)
	if c.Status != checkOK {
		t.Errorf("Status = %s, want ok when no hooks/MCP items; message: %s", c.Status, c.Message)
	}
}

func TestRunDoctorFixForce_RelinksDeadProviderLink(t *testing.T) {
	resetDoctorFlags(t)

	home := t.TempDir()
	t.Setenv("HOME", home)
	contentDir := filepath.Join(home, ".syllago", "content")
	skillDir := filepath.Join(contentDir, "skills", "repair-me")
	writeDoctorCmdFile(t, filepath.Join(skillDir, "SKILL.md"), "# Repair Me\n")

	installDir := filepath.Join(home, ".claude", "skills")
	linkPath := filepath.Join(installDir, "repair-me")
	oldTarget := filepath.Join(home, ".syllago", "content", "skills", "gone")
	symlinkDoctorCmd(t, oldTarget, linkPath)

	withDoctorCmdGlobals(t, home, contentDir, []provider.Provider{
		doctorCmdProvider("claude-code", map[catalog.ContentType]string{catalog.Skills: installDir}),
	})

	stdout, _ := output.SetForTest(t)
	captureOsExit(t)
	mustSetDoctorFlag(t, "fix", "true")
	mustSetDoctorFlag(t, "force", "true")

	if err := doctorCmd.RunE(doctorCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != skillDir {
		t.Fatalf("target = %q, want %q", target, skillDir)
	}
	if !strings.Contains(stdout.String(), "relink: "+linkPath+" -> "+skillDir) {
		t.Fatalf("output missing relink plan:\n%s", stdout.String())
	}
}

func TestRunDoctorFixForce_PrunesDeadProviderLinkWithoutLibraryMatch(t *testing.T) {
	resetDoctorFlags(t)

	home := t.TempDir()
	t.Setenv("HOME", home)
	contentDir := filepath.Join(home, ".syllago", "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	installDir := filepath.Join(home, ".claude", "skills")
	linkPath := filepath.Join(installDir, "gone")
	oldTarget := filepath.Join(home, ".syllago", "content", "skills", "gone")
	symlinkDoctorCmd(t, oldTarget, linkPath)

	withDoctorCmdGlobals(t, home, contentDir, []provider.Provider{
		doctorCmdProvider("claude-code", map[catalog.ContentType]string{catalog.Skills: installDir}),
	})

	stdout, _ := output.SetForTest(t)
	captureOsExit(t)
	mustSetDoctorFlag(t, "fix", "true")
	mustSetDoctorFlag(t, "force", "true")

	if err := doctorCmd.RunE(doctorCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Fatalf("Lstat err = %v, want link removed", err)
	}
	if !strings.Contains(stdout.String(), "prune:  "+linkPath+" (target gone: "+oldTarget+")") {
		t.Fatalf("output missing prune plan:\n%s", stdout.String())
	}
}

func TestRunDoctorFix_NothingBroken(t *testing.T) {
	resetDoctorFlags(t)

	home := t.TempDir()
	t.Setenv("HOME", home)
	contentDir := filepath.Join(home, ".syllago", "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	installDir := filepath.Join(home, ".claude", "skills")
	withDoctorCmdGlobals(t, home, contentDir, []provider.Provider{
		doctorCmdProvider("claude-code", map[catalog.ContentType]string{catalog.Skills: installDir}),
	})

	stdout, _ := output.SetForTest(t)
	captureOsExit(t)
	mustSetDoctorFlag(t, "fix", "true")

	if err := doctorCmd.RunE(doctorCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	if !strings.Contains(stdout.String(), "Nothing to fix.") {
		t.Fatalf("output missing nothing-to-fix message:\n%s", stdout.String())
	}
}

func resetDoctorFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		_ = doctorCmd.Flags().Set("fix", "false")
		_ = doctorCmd.Flags().Set("force", "false")
	})
}

func mustSetDoctorFlag(t *testing.T, name, value string) {
	t.Helper()
	if err := doctorCmd.Flags().Set(name, value); err != nil {
		t.Fatalf("setting --%s: %v", name, err)
	}
}

func withDoctorCmdGlobals(t *testing.T, projectRoot, contentDir string, providers []provider.Provider) {
	t.Helper()

	origContentDir := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = contentDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origContentDir })

	origProviders := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = providers
	t.Cleanup(func() { provider.AllProviders = origProviders })

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })
}

func doctorCmdProvider(slug string, dirs map[catalog.ContentType]string) provider.Provider {
	return provider.Provider{
		Name:   slug,
		Slug:   slug,
		Detect: func(string) bool { return true },
		InstallDir: func(home string, ct catalog.ContentType) string {
			return dirs[ct]
		},
	}
}

func writeDoctorCmdFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func symlinkDoctorCmd(t *testing.T, target, linkPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
}
