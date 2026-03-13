package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestCreateLoadoutScreenSmoke(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("starts at types step when pre-filled", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		if s.step != clStepTypes {
			t.Errorf("step = %v, want clStepTypes", s.step)
		}
	})
	t.Run("starts at provider step when no prefill", func(t *testing.T) {
		s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
		if s.step != clStepProvider {
			t.Errorf("step = %v, want clStepProvider", s.step)
		}
	})
	t.Run("split view initialized with wiz prefix", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		if s.splitView.zonePrefix != "wiz" {
			t.Errorf("splitView.zonePrefix = %q, want \"wiz\"", s.splitView.zonePrefix)
		}
	})
	t.Run("dest options include registry when scoped", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "my-reg", providers, cat, 80, 30)
		if len(s.destOptions) != 3 {
			t.Errorf("destOptions = %d, want 3 with registry scope", len(s.destOptions))
		}
	})
}

func TestCreateLoadoutScreenUpdate(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("enter on provider step advances to types", func(t *testing.T) {
		s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter)
		if s.step != clStepTypes {
			t.Errorf("step = %v, want clStepTypes after Enter", s.step)
		}
	})

	t.Run("esc on provider step does not crash", func(t *testing.T) {
		s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEsc)
		if s.confirmed {
			t.Error("confirmed should be false after Esc")
		}
	})

	t.Run("esc on types step goes back to provider", func(t *testing.T) {
		s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
		s.step = clStepTypes
		s, _ = s.Update(keyEsc)
		if s.step != clStepProvider {
			t.Errorf("step = %v, want clStepProvider after Esc on types", s.step)
		}
	})

	t.Run("space on types toggles checkbox", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		if len(s.typeEntries) == 0 {
			t.Skip("no type entries")
		}
		initial := s.typeEntries[0].checked
		s, _ = s.Update(keySpace)
		if s.typeEntries[0].checked == initial {
			t.Error("space should toggle type checkbox")
		}
	})

	t.Run("enter on types with none selected shows error", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		// Uncheck all
		for i := range s.typeEntries {
			s.typeEntries[i].checked = false
		}
		s, _ = s.Update(keyEnter)
		if s.step != clStepTypes {
			t.Error("should stay on types step when none selected")
		}
		if s.message != "Select at least one content type" {
			t.Errorf("message = %q, want validation message", s.message)
		}
	})

	t.Run("enter on types advances to items", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter)
		if s.step != clStepItems {
			t.Errorf("step = %v, want clStepItems", s.step)
		}
	})

	t.Run("space on items step toggles selection", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // advance to items
		if s.step != clStepItems {
			t.Fatalf("expected clStepItems, got %d", s.step)
		}
		filtered := s.filteredTypeItems()
		if len(filtered) == 0 {
			t.Skip("no items")
		}
		idx := filtered[0]
		initial := s.entries[idx].selected
		s, _ = s.Update(keySpace)
		if s.entries[idx].selected == initial {
			t.Error("space should toggle item selection")
		}
	})

	t.Run("t key toggles showAllCompat", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepItems
		s.buildTypeItemMaps()
		initial := s.showAllCompat
		s, _ = s.Update(keyRune('t'))
		if s.showAllCompat == initial {
			t.Error("t should toggle showAllCompat")
		}
	})

	t.Run("slash activates search", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepItems
		s.buildTypeItemMaps()
		s, _ = s.Update(keyRune('/'))
		if !s.searchActive {
			t.Error("/ should activate search")
		}
	})

	t.Run("esc clears search when active", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepItems
		s.buildTypeItemMaps()
		s.searchActive = true
		s.searchInput.Focus()
		s, _ = s.Update(keyEsc)
		if s.searchActive {
			t.Error("Esc should deactivate search")
		}
	})

	t.Run("l focuses preview pane", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepItems
		s.buildTypeItemMaps()
		s, _ = s.Update(keyRune('l'))
		if s.splitView.focusedPane != panePreview {
			t.Error("l should focus preview pane")
		}
	})

	t.Run("h focuses list pane", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepItems
		s.buildTypeItemMaps()
		s.splitView.focusedPane = panePreview
		s, _ = s.Update(keyRune('h'))
		if s.splitView.focusedPane != paneList {
			t.Error("h should focus list pane")
		}
	})

	t.Run("enter advances through types to name", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		nTypes := len(s.selectedTypes)
		for i := 0; i < nTypes; i++ {
			s, _ = s.Update(keyEnter)
		}
		if s.step != clStepName {
			t.Errorf("step = %v, want clStepName after all types", s.step)
		}
	})

	t.Run("tab switches focus between name and desc", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		if !s.nameFirst {
			t.Fatal("nameFirst should be true initially")
		}
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
		if s.nameFirst {
			t.Error("Tab should switch to desc")
		}
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
		if !s.nameFirst {
			t.Error("Tab should switch back to name")
		}
	})

	t.Run("enter on name with empty shows error", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		s.nameInput.SetValue("")
		s, _ = s.Update(keyEnter)
		if s.step != clStepName {
			t.Error("should stay on name step with empty name")
		}
		if s.message != "Name is required" {
			t.Errorf("message = %q", s.message)
		}
	})

	t.Run("enter on name advances to dest", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		s.nameInput.SetValue("my-loadout")
		s, _ = s.Update(keyEnter)
		if s.step != clStepDest {
			t.Errorf("step = %v, want clStepDest", s.step)
		}
	})

	t.Run("enter on dest advances to review", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepDest
		s, _ = s.Update(keyEnter)
		if s.step != clStepReview {
			t.Errorf("step = %v, want clStepReview", s.step)
		}
		if s.reviewBtnCursor != 1 {
			t.Error("reviewBtnCursor should default to 1 (Create)")
		}
	})

	t.Run("review enter on Create confirms", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepReview
		s.reviewBtnCursor = 1
		s, _ = s.Update(keyEnter)
		if !s.confirmed {
			t.Error("confirmed should be true after Create")
		}
	})

	t.Run("review enter on Back returns to dest", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepReview
		s.reviewBtnCursor = 0
		s, _ = s.Update(keyEnter)
		if s.step != clStepDest {
			t.Errorf("step = %v, want clStepDest", s.step)
		}
	})

	t.Run("left/right switches review buttons", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepReview
		s.reviewBtnCursor = 1
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyLeft})
		if s.reviewBtnCursor != 0 {
			t.Errorf("left should move to Back (0), got %d", s.reviewBtnCursor)
		}
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
		if s.reviewBtnCursor != 1 {
			t.Errorf("right should move to Create (1), got %d", s.reviewBtnCursor)
		}
	})

	t.Run("previewCmdForCursor returns valid cmd", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		cmd := s.previewCmdForCursor()
		if cmd == nil {
			// Could be nil if no items have files — that's OK
			return
		}
		msg := cmd()
		if _, ok := msg.(splitViewCursorMsg); !ok {
			t.Errorf("expected splitViewCursorMsg, got %T", msg)
		}
	})
}

func TestCreateLoadoutScreenView(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("types step renders breadcrumb", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		got := s.View()
		if got == "" {
			t.Error("View() returned empty string")
		}
		if !strings.Contains(got, "Create") {
			t.Error("breadcrumb should contain Create")
		}
	})

	t.Run("types step shows danger badges", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.typeEntries = []typeCheckEntry{
			{ct: catalog.Hooks, checked: true, count: 1},
			{ct: catalog.MCP, checked: true, count: 1},
			{ct: catalog.Skills, checked: true, count: 2},
		}
		got := s.View()
		if !strings.Contains(got, "!!") {
			t.Error("types step should show !! badge for Hooks and MCP")
		}
	})

	t.Run("items step shows content type label", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // advance to items
		got := s.View()
		if got == "" {
			t.Error("View() returned empty string")
		}
	})

	t.Run("name step renders inputs", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		got := s.View()
		if !strings.Contains(got, "Name your loadout") {
			t.Error("name step should show name prompt")
		}
	})

	t.Run("dest step renders options", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepDest
		got := s.View()
		if !strings.Contains(got, "destination") {
			t.Error("dest step should show destination prompt")
		}
	})

	t.Run("review step renders summary", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.nameInput.SetValue("my-loadout")
		s.step = clStepReview
		got := s.View()
		if !strings.Contains(got, "my-loadout") {
			t.Error("review should show loadout name")
		}
		if !strings.Contains(got, "Review") {
			t.Error("review should show Review heading")
		}
	})

	t.Run("review with security content shows notice", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.nameInput.SetValue("test")
		for i := range s.entries {
			if s.entries[i].item.Type == catalog.Hooks {
				s.entries[i].selected = true
			}
		}
		s.step = clStepReview
		got := s.View()
		if !strings.Contains(got, "Security Notice") {
			t.Error("review should show security warning when hooks selected")
		}
	})
}

func TestCreateLoadoutScreenSplitView(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("wide terminal shows separator", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 120, 40)
		s.step = clStepItems
		s.buildTypeItemMaps()
		got := s.View()
		if !strings.Contains(got, "│") {
			t.Error("wide terminal should show split-view separator")
		}
	})

	t.Run("wide terminal shows title bar", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 120, 40)
		s.step = clStepItems
		s.buildTypeItemMaps()
		got := s.View()
		if !strings.Contains(got, "Items") || !strings.Contains(got, "Preview") {
			t.Error("should show Items | Preview title bar")
		}
	})

	t.Run("narrow terminal single pane", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 60, 20)
		s.step = clStepItems
		s.buildTypeItemMaps()
		got := s.View()
		if got == "" {
			t.Error("narrow terminal View() should not be empty")
		}
	})
}

// ---------------------------------------------------------------------------
// Golden file tests
// ---------------------------------------------------------------------------

// navigateToCreateLoadout navigates from homepage to the create loadout screen.
func navigateToCreateLoadout(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+1) // Loadouts in sidebar
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenLoadoutCards)

	// Press 'a' to open create loadout
	m, _ = app.Update(keyRune('a'))
	app = m.(App)
	assertScreen(t, app, screenCreateLoadout)
	return app
}

func navigateToCreateLoadoutSize(t *testing.T, width, height int) App {
	t.Helper()
	app := testAppSize(t, width, height)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+1)
	m, _ := app.Update(keyEnter)
	app = m.(App)

	m, _ = app.Update(keyRune('a'))
	app = m.(App)
	assertScreen(t, app, screenCreateLoadout)
	return app
}

func TestGoldenFullApp_CreateLoadoutTypes(t *testing.T) {
	app := navigateToCreateLoadout(t)
	// Should start at types step (provider is pre-filled from loadout cards context)
	requireGolden(t, "fullapp-create-loadout-types", snapshotApp(t, app))
}

func TestGoldenFullApp_CreateLoadoutItems(t *testing.T) {
	app := navigateToCreateLoadout(t)
	// Advance to items step
	m, _ := app.Update(keyEnter)
	app = m.(App)
	requireGolden(t, "fullapp-create-loadout-items", snapshotApp(t, app))
}

func TestGoldenSized_CreateLoadoutTypes(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := navigateToCreateLoadoutSize(t, sz.width, sz.height)
			requireGolden(t, "fullapp-create-loadout-types-"+sz.tag,
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}

// TestCreateLoadoutRescanFindsNewLoadout verifies that after doCreateLoadoutFromScreen
// writes a loadout to the project destination, the catalog rescan finds it.
// This is the regression test for the creation bug where loadouts didn't appear.
func TestCreateLoadoutRescanFindsNewLoadout(t *testing.T) {
	app := testApp(t)
	contentRoot := app.catalog.RepoRoot

	// Count loadouts before creation
	beforeCount := len(app.catalog.ByType(catalog.Loadouts))

	// Set up the screen as if the user completed the wizard
	scr := newCreateLoadoutScreen("claude-code", "", app.providers, app.catalog, 80, 30)
	scr.nameInput.SetValue("new-test-loadout")
	scr.descInput.SetValue("A test loadout")
	scr.step = clStepDest
	scr.destCursor = 0 // Project destination
	scr.confirmed = true

	// Execute doCreateLoadoutFromScreen (the tea.Cmd) synchronously
	cmd := app.doCreateLoadoutFromScreen(scr)
	msg := cmd()

	// Verify no error
	result := msg.(doCreateLoadoutMsg)
	if result.err != nil {
		t.Fatalf("doCreateLoadout failed: %v", result.err)
	}

	// Verify the loadout.yaml was written to contentRoot (not projectRoot)
	loadoutPath := filepath.Join(contentRoot, "loadouts", "claude-code", "new-test-loadout", "loadout.yaml")
	if _, err := os.Stat(loadoutPath); err != nil {
		t.Fatalf("loadout.yaml not found at %s: %v", loadoutPath, err)
	}

	// Simulate the doCreateLoadoutMsg handler: rescan the catalog
	cat, err := catalog.Scan(contentRoot, contentRoot)
	if err != nil {
		t.Fatalf("rescan failed: %v", err)
	}

	afterCount := len(cat.ByType(catalog.Loadouts))
	if afterCount != beforeCount+1 {
		t.Errorf("loadout count after rescan = %d, want %d (before=%d)", afterCount, beforeCount+1, beforeCount)
	}

	// Verify the new loadout is in the catalog
	found := false
	for _, item := range cat.ByType(catalog.Loadouts) {
		if item.Name == "new-test-loadout" {
			found = true
			if item.Provider != "claude-code" {
				t.Errorf("provider = %q, want claude-code", item.Provider)
			}
			break
		}
	}
	if !found {
		t.Error("new-test-loadout not found in catalog after rescan")
	}
}
