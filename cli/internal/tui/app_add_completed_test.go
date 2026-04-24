package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// TestApp_HandlesAddCompletedMsgViaRefreshContent verifies that when the
// monolithic add wizard finishes writing rules, the resulting addCompletedMsg
// triggers a catalog rescan + refreshContent so the library view picks up the
// newly-written rules without requiring a manual [R] press (D18, Task 4.10).
func TestApp_HandlesAddCompletedMsgViaRefreshContent(t *testing.T) {
	t.Parallel()

	// Build an App rooted at a real content directory so rescanCatalog has
	// something to scan after addCompletedMsg fires.
	contentRoot := t.TempDir()
	app := NewApp(&catalog.Catalog{}, nil, "0.0.0-test", false, nil, &config.Config{}, false, contentRoot, "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Sanity: library empty before.
	if got := len(a.catalog.Items); got != 0 {
		t.Fatalf("expected empty catalog before addCompletedMsg, got %d items", got)
	}

	// Dispatch addCompletedMsg. Handler must return a non-nil cmd that runs
	// rescanCatalog — firing a catalogReadyMsg (async). The cmd is what wires
	// the wizard completion into the catalog refresh path; the rescan itself
	// is exercised by other tests.
	updated, cmd := a.Update(addCompletedMsg{count: 1})
	_ = updated.(App) // guard: handler must not swap App types
	if cmd == nil {
		t.Fatalf("expected non-nil cmd from addCompletedMsg handler (rescanCatalog cmd)")
	}
	msg := cmd()
	if _, ok := msg.(catalogReadyMsg); !ok {
		t.Errorf("expected addCompletedMsg handler to return rescanCatalog cmd (catalogReadyMsg), got %T", msg)
	}
}
