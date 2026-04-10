// cli/internal/tui/golden_components_test.go
package tui_v1

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestGoldenComponent_Sidebar(t *testing.T) {
	cat := testCatalog(t)
	m := newSidebarModel(cat, "1.0.0", 0)
	// sidebarModel has no width field — width is the package-level sidebarWidth constant (18).
	m.height = 30
	m.focused = true
	requireGolden(t, "component-sidebar", stripANSI(m.View()))
}

func TestGoldenComponent_Items(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)
	items := cat.ByType(catalog.Skills)
	m := newItemsModel(catalog.Skills, items, providers, cat.RepoRoot)
	m.width = 62 // content panel width = 80 - sidebarWidth(18) = 62
	m.height = 28
	requireGolden(t, "component-items", normalizeSnapshot(stripANSI(m.View())))
}

func TestGoldenComponent_DetailTabs(t *testing.T) {
	// Navigate to detail to get a fully-initialized detailModel with tab bar rendered.
	app := navigateToDetail(t, catalog.Skills)
	// Snapshot just the detail view (content panel only)
	requireGolden(t, "component-detail-tabs", normalizeSnapshot(stripANSI(app.detail.View())))
}

func TestGoldenComponent_Modal(t *testing.T) {
	m := newConfirmModal("Confirm Uninstall", "Remove alpha-skill from all providers?")
	requireGolden(t, "component-modal", stripANSI(m.View()))
}

func TestGoldenComponent_HelpOverlay(t *testing.T) {
	m := helpOverlayModel{active: true}
	// Use screenCategory context for the help overlay
	requireGolden(t, "component-help", stripANSI(m.View(screenCategory)))
}

func TestGoldenComponent_FileBrowser(t *testing.T) {
	// Use a controlled temp dir with stable subdirectory names to avoid
	// non-determinism from random temp dir paths in the rendered output.
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "skills", "alpha-skill"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "skills", "beta-skill"), 0o755)

	fb := newFileBrowser(tmp, catalog.Skills)
	fb.width = 62
	fb.height = 28
	requireGolden(t, "component-filebrowser", normalizeSnapshot(stripANSI(fb.View())))
}
