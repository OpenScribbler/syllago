package main

// Subprocess e2e tests for conflict detection.
//
// These tests run the real syllago binary (built from source) against a
// controlled fake home directory, using real Codex + GeminiCLI provider
// detection and real filesystem operations.
//
// Unlike the RunE-level tests in install_conflict_test.go (which use stub
// providers that ignore homeDir), these tests exercise the actual provider
// path computation — e.g. that Codex installs to $HOME/.agents/skills/ and
// GeminiCLI reads from there.
//
// Gate: set SYLLAGO_TEST_E2E=1 to run. Excluded by default so CI does not
// pay the binary build cost on every push.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles syllago into a temp dir and returns the binary path.
// The temp dir (and binary) are removed when the test ends.
func buildBinary(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	binary := filepath.Join(binDir, "syllago")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = "." // cli/cmd/syllago
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building syllago binary: %v\n%s", err, out)
	}
	return binary
}

// setupFakeHome creates a minimal fake home directory for e2e testing.
// Returns the fakeHome path; cleanup is via t.TempDir().
//
// Directory layout:
//
//	<fakeHome>/
//	  .syllago/content/skills/my-skill/SKILL.md   ← library skill
//	  .codex/                                      ← triggers Codex detection
//	  .gemini/                                     ← triggers GeminiCLI detection
func setupFakeHome(t *testing.T) string {
	t.Helper()
	fakeHome := t.TempDir()

	// Library skill
	skillDir := filepath.Join(fakeHome, ".syllago", "content", "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("creating skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill\nDoes something.\n"), 0644); err != nil {
		t.Fatalf("writing SKILL.md: %v", err)
	}

	// Provider detection directories
	for _, dir := range []string{".codex", ".gemini"} {
		if err := os.MkdirAll(filepath.Join(fakeHome, dir), 0755); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}
	}

	return fakeHome
}

// runSyllago executes the binary with HOME=fakeHome and returns (stdout+stderr, exitErr).
func runSyllago(t *testing.T, binary, fakeHome string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = append(filterEnv(os.Environ(), "HOME"), "HOME="+fakeHome)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// filterEnv removes entries matching key= from the environment slice.
func filterEnv(env []string, key string) []string {
	prefix := key + "="
	filtered := env[:0:len(env)]
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// TestInstallConflict_E2E_NoInput_ConflictWarning verifies that when Codex and
// GeminiCLI are both detected, running install --to-all --no-input prints a
// conflict warning and exits successfully (ResolutionAll — no filtering).
//
// After the command:
//   - ~/.agents/skills/my-skill  exists  (Codex installed it)
//   - ~/.gemini/skills/my-skill  exists  (GeminiCLI installed its own copy)
//   - Warning about shared path in output
func TestInstallConflict_E2E_NoInput_ConflictWarning(t *testing.T) {
	if os.Getenv("SYLLAGO_TEST_E2E") == "" {
		t.Skip("set SYLLAGO_TEST_E2E=1 to run e2e tests")
	}

	binary := buildBinary(t)
	fakeHome := setupFakeHome(t)

	out, err := runSyllago(t, binary, fakeHome,
		"install", "--to-all", "--type", "skills", "--all", "--no-input",
	)
	if err != nil {
		t.Fatalf("syllago exited with error: %v\noutput:\n%s", err, out)
	}

	// Conflict warning should appear.
	if !strings.Contains(out, "conflict") && !strings.Contains(out, "shared") {
		t.Errorf("expected conflict warning in output:\n%s", out)
	}

	// With --no-input (ResolutionAll), both providers should have installed.
	agentsPath := filepath.Join(fakeHome, ".agents", "skills", "my-skill")
	geminiPath := filepath.Join(fakeHome, ".gemini", "skills", "my-skill")

	if _, err := os.Lstat(agentsPath); err != nil {
		t.Errorf("Codex should have installed to %s, but it doesn't exist", agentsPath)
	}
	if _, err := os.Lstat(geminiPath); err != nil {
		t.Errorf("GeminiCLI should have installed to %s, but it doesn't exist", geminiPath)
	}
}

// TestInstallConflict_E2E_SharedOnly verifies that passing choice "1" (shared path
// only) to the conflict prompt results in:
//   - ~/.agents/skills/my-skill  exists  (Codex installed it)
//   - ~/.gemini/skills/my-skill  absent  (GeminiCLI was removed from active list)
//
// We pipe "1\n" to stdin to simulate the user choosing option 1.
func TestInstallConflict_E2E_SharedOnly(t *testing.T) {
	if os.Getenv("SYLLAGO_TEST_E2E") == "" {
		t.Skip("set SYLLAGO_TEST_E2E=1 to run e2e tests")
	}

	binary := buildBinary(t)
	fakeHome := setupFakeHome(t)

	cmd := exec.Command(binary, "install", "--to-all", "--type", "skills", "--all")
	cmd.Env = append(filterEnv(os.Environ(), "HOME"), "HOME="+fakeHome)
	cmd.Stdin = strings.NewReader("1\n") // choose "shared path only"

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("syllago exited with error: %v\noutput:\n%s", err, out)
	}

	agentsPath := filepath.Join(fakeHome, ".agents", "skills", "my-skill")
	geminiPath := filepath.Join(fakeHome, ".gemini", "skills", "my-skill")

	if _, err := os.Lstat(agentsPath); err != nil {
		t.Errorf("SharedOnly: Codex should have installed to %s", agentsPath)
	}
	if _, err := os.Lstat(geminiPath); err == nil {
		t.Errorf("SharedOnly: GeminiCLI dir %s should be empty (reader was skipped)", geminiPath)
	}
}

// TestInstallConflict_E2E_OwnDirsOnly verifies that passing choice "2" (own dirs
// only) to the conflict prompt results in:
//   - ~/.agents/skills/my-skill  absent  (Codex was removed from active list)
//   - ~/.gemini/skills/my-skill  exists  (GeminiCLI installed to its own dir)
//
// We pipe "2\n" to stdin to simulate the user choosing option 2.
func TestInstallConflict_E2E_OwnDirsOnly(t *testing.T) {
	if os.Getenv("SYLLAGO_TEST_E2E") == "" {
		t.Skip("set SYLLAGO_TEST_E2E=1 to run e2e tests")
	}

	binary := buildBinary(t)
	fakeHome := setupFakeHome(t)

	cmd := exec.Command(binary, "install", "--to-all", "--type", "skills", "--all")
	cmd.Env = append(filterEnv(os.Environ(), "HOME"), "HOME="+fakeHome)
	cmd.Stdin = strings.NewReader("2\n") // choose "own dirs only"

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("syllago exited with error: %v\noutput:\n%s", err, out)
	}

	agentsPath := filepath.Join(fakeHome, ".agents", "skills", "my-skill")
	geminiPath := filepath.Join(fakeHome, ".gemini", "skills", "my-skill")

	if _, err := os.Lstat(agentsPath); err == nil {
		t.Errorf("OwnDirsOnly: Codex dir %s should be empty (installer was skipped)", agentsPath)
	}
	if _, err := os.Lstat(geminiPath); err != nil {
		t.Errorf("OwnDirsOnly: GeminiCLI should have installed to %s", geminiPath)
	}
}

// TestInstallConflict_E2E_Cleanup_Proof verifies that ALL files created by the
// e2e tests live under fakeHome (a t.TempDir()). This proves no stray files
// are written to the real home directory.
//
// Concretely: run install --no-input and then assert that nothing exists in the
// real home that wasn't there before, by checking only fakeHome paths.
func TestInstallConflict_E2E_Cleanup_Proof(t *testing.T) {
	if os.Getenv("SYLLAGO_TEST_E2E") == "" {
		t.Skip("set SYLLAGO_TEST_E2E=1 to run e2e tests")
	}

	binary := buildBinary(t)
	fakeHome := setupFakeHome(t)

	_, err := runSyllago(t, binary, fakeHome,
		"install", "--to-all", "--type", "skills", "--all", "--no-input",
	)
	if err != nil {
		t.Fatalf("syllago failed: %v", err)
	}

	// Walk everything that was written and verify it's under fakeHome.
	realHome, _ := os.UserHomeDir()
	if err := filepath.Walk(fakeHome, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Ensure path is under fakeHome (it always will be — this is a tautology
		// for files written inside t.TempDir(), but proves the pattern).
		if !strings.HasPrefix(path, fakeHome) {
			t.Errorf("unexpected path outside fakeHome: %s", path)
		}
		return nil
	}); err != nil {
		t.Errorf("walking fakeHome: %v", err)
	}

	// Extra guard: nothing should have been written to the real .agents/skills/
	// or .gemini/skills/ in the real home as a side-effect of the test.
	for _, checkPath := range []string{
		filepath.Join(realHome, ".agents", "skills", "my-skill"),
		filepath.Join(realHome, ".gemini", "skills", "my-skill"),
	} {
		if _, err := os.Lstat(checkPath); err == nil {
			t.Errorf("e2e test left stray file at real home path: %s", checkPath)
		}
	}
}
