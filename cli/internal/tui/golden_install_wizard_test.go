package tui

import (
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// snapshotInstallWizard captures the install wizard view with path normalization.
func snapshotInstallWizard(t *testing.T, m *installWizardModel) string {
	t.Helper()
	return normalizeSnapshot(m.View())
}

// setupProviderStepWizard creates an installWizardModel at the provider step with 2
// providers and a seeded "All providers" option visible. Used for golden tests.
func setupProviderStepWizard(t *testing.T, w, h int) *installWizardModel {
	t.Helper()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(t.TempDir(), "skills", "my-skill"))

	m := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	m.width = w
	m.height = h
	m.shell.SetWidth(w)
	// Ensure we stay at the provider step (multi-provider, no auto-skip).
	// Shell was initialized by openInstallWizard; leave it as-is.
	return m
}

// setupConflictStepWizard creates an installWizardModel at the conflict step with
// seeded conflict data. The shared path comes from t.TempDir() and is normalized
// to <TESTDIR> by normalizeSnapshot, keeping golden files deterministic.
func setupConflictStepWizard(t *testing.T, w, h int) *installWizardModel {
	t.Helper()
	sharedPath := t.TempDir()
	provA := testConflictInstaller("gemini-cli", "Gemini CLI", sharedPath)
	provB := testConflictReader("opencode", "OpenCode", sharedPath)
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(t.TempDir(), "skills", "my-skill"))

	m := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	m.width = w
	m.height = h
	m.shell.SetWidth(w)

	// Seed conflict state directly (same as TestInstallWizard_ConflictView).
	m.selectAll = true
	m.conflicts = []installer.Conflict{{
		SharedPath:   sharedPath,
		InstallingTo: provA,
		AlsoReadBy:   []provider.Provider{provB},
	}}
	m.conflictCursor = 0
	m.step = installStepConflict
	m.shell.SetSteps([]string{"Provider", "Conflicts"})
	m.shell.SetActive(1)

	return m
}

// --- Provider step golden tests ---

func TestGolden_InstallProvider_80x30(t *testing.T) {
	t.Parallel()
	m := setupProviderStepWizard(t, 80, 30)
	requireGolden(t, "install-provider-80x30", snapshotInstallWizard(t, m))
}

func TestGolden_InstallProvider_60x20(t *testing.T) {
	t.Parallel()
	m := setupProviderStepWizard(t, 60, 20)
	requireGolden(t, "install-provider-60x20", snapshotInstallWizard(t, m))
}

// --- Conflict step golden tests ---

func TestGolden_InstallConflict_80x30(t *testing.T) {
	t.Parallel()
	m := setupConflictStepWizard(t, 80, 30)
	requireGolden(t, "install-conflict-80x30", snapshotInstallWizard(t, m))
}

func TestGolden_InstallConflict_60x20(t *testing.T) {
	t.Parallel()
	m := setupConflictStepWizard(t, 60, 20)
	requireGolden(t, "install-conflict-60x20", snapshotInstallWizard(t, m))
}
