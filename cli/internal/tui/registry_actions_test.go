package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// ---------------------------------------------------------------------------
// Helper: create an App with pre-configured registries
// ---------------------------------------------------------------------------

// testAppWithRegistries creates a test App that has 2 registries configured,
// then navigates to the registries screen.
func testAppWithRegistries(t *testing.T) App {
	t.Helper()
	cat := testCatalog(t)
	providers := testProviders(t)
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "test-registry", URL: "https://github.com/example/test-registry.git"},
			{Name: "other-registry", URL: "https://github.com/example/other-registry.git"},
		},
	}

	app := NewApp(cat, providers, "1.0.0", false, nil, cfg, false, cat.RepoRoot)
	app.width = 80
	app.height = 30
	app.items.width = 80
	app.items.height = 30
	app.detail.width = 80
	app.detail.height = 30
	app.settings.width = 80
	app.settings.height = 30
	app.importer.width = 80
	app.importer.height = 30
	app.updater.width = 80
	app.updater.height = 30

	// Navigate to registries screen
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+2) // Registries
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenRegistries)
	return app
}

// ---------------------------------------------------------------------------
// Registry Add Modal Tests
// ---------------------------------------------------------------------------

func TestRegistryAddModalOpenClose(t *testing.T) {
	t.Run("a opens add modal", func(t *testing.T) {
		app := navigateToRegistries(t)
		m, _ := app.Update(keyRune('a'))
		app = m.(App)

		if !app.registryAddModal.active {
			t.Fatal("expected registry add modal to be active after pressing 'a'")
		}
		if app.focus != focusModal {
			t.Fatalf("expected focusModal, got %d", app.focus)
		}
	})

	t.Run("esc closes add modal", func(t *testing.T) {
		app := navigateToRegistries(t)
		m, _ := app.Update(keyRune('a'))
		app = m.(App)

		m, _ = app.Update(keyEsc)
		app = m.(App)

		if app.registryAddModal.active {
			t.Fatal("expected registry add modal to be closed after Esc")
		}
		if app.focus != focusContent {
			t.Fatalf("expected focusContent after modal close, got %d", app.focus)
		}
	})

	t.Run("cancel button closes add modal", func(t *testing.T) {
		app := navigateToRegistries(t)
		m, _ := app.Update(keyRune('a'))
		app = m.(App)

		// Tab to buttons (url→name→buttons)
		m, _ = app.Update(keyTab)
		app = m.(App)
		m, _ = app.Update(keyTab)
		app = m.(App)
		// Move to Cancel button (right)
		m, _ = app.Update(tea.KeyMsg{Type: tea.KeyRight})
		app = m.(App)
		// Press Enter on Cancel
		m, _ = app.Update(keyEnter)
		app = m.(App)

		if app.registryAddModal.active {
			t.Fatal("expected registry add modal to be closed after Cancel")
		}
	})
}

func TestRegistryAddModalValidation(t *testing.T) {
	t.Run("empty URL shows error", func(t *testing.T) {
		app := navigateToRegistries(t)
		m, _ := app.Update(keyRune('a'))
		app = m.(App)

		// Press Enter without typing a URL (btnCursor defaults to 0 = Add)
		m, _ = app.Update(keyEnter)
		app = m.(App)

		if !app.registryAddModal.active {
			t.Fatal("modal should stay active on empty URL")
		}
		if app.registryAddModal.message != "URL is required" {
			t.Fatalf("expected 'URL is required' error, got %q", app.registryAddModal.message)
		}
	})

	t.Run("invalid name shows error", func(t *testing.T) {
		app := testAppWithRegistries(t)
		m, _ := app.Update(keyRune('a'))
		app = m.(App)

		// Type a URL that produces an invalid name
		for _, r := range "https://example.com" {
			m, _ = app.Update(keyRune(r))
			app = m.(App)
		}
		// Tab to name field, type invalid name with spaces
		m, _ = app.Update(keyTab)
		app = m.(App)
		for _, r := range "bad name!" {
			m, _ = app.Update(keyRune(r))
			app = m.(App)
		}
		// Confirm
		m, _ = app.Update(keyEnter)
		app = m.(App)

		if !app.registryAddModal.active {
			t.Fatal("modal should stay active on invalid name")
		}
		if !app.registryAddModal.messageIsErr {
			t.Fatal("expected error message for invalid name")
		}
		if !strings.Contains(app.registryAddModal.message, "Invalid registry name") {
			t.Fatalf("expected 'Invalid registry name' error, got %q", app.registryAddModal.message)
		}
	})

	t.Run("duplicate name shows error", func(t *testing.T) {
		app := testAppWithRegistries(t)
		m, _ := app.Update(keyRune('a'))
		app = m.(App)

		// Type a URL
		for _, r := range "https://github.com/whatever/repo.git" {
			m, _ = app.Update(keyRune(r))
			app = m.(App)
		}
		// Tab to name field and type a name that matches existing registry
		m, _ = app.Update(keyTab)
		app = m.(App)
		for _, r := range "test-registry" {
			m, _ = app.Update(keyRune(r))
			app = m.(App)
		}
		m, _ = app.Update(keyEnter)
		app = m.(App)

		if !app.registryAddModal.active {
			t.Fatal("modal should stay active on duplicate name")
		}
		if !app.registryAddModal.messageIsErr {
			t.Fatal("expected error message for duplicate")
		}
		if !strings.Contains(app.registryAddModal.message, "already exists") {
			t.Fatalf("expected 'already exists' error, got %q", app.registryAddModal.message)
		}
	})
}

func TestRegistryAddModalTabSwitchesFields(t *testing.T) {
	app := navigateToRegistries(t)
	m, _ := app.Update(keyRune('a'))
	app = m.(App)

	if app.registryAddModal.focusedField != 0 {
		t.Fatal("URL field should be focused initially")
	}

	m, _ = app.Update(keyTab)
	app = m.(App)
	if app.registryAddModal.focusedField != 1 {
		t.Fatal("Name field should be focused after first Tab")
	}

	m, _ = app.Update(keyTab)
	app = m.(App)
	if app.registryAddModal.focusedField != 2 {
		t.Fatal("Buttons should be focused after second Tab")
	}

	m, _ = app.Update(keyTab)
	app = m.(App)
	if app.registryAddModal.focusedField != 0 {
		t.Fatal("URL field should be focused after third Tab (wraps around)")
	}
}

// ---------------------------------------------------------------------------
// Registry Add Done Message Tests
// ---------------------------------------------------------------------------

func TestRegistryAddDoneMsg(t *testing.T) {
	t.Run("success shows toast", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registryAddDoneMsg{name: "new-reg"})
		app = m.(App)

		if app.registryOpInProgress {
			t.Fatal("registryOpInProgress should be false after done msg")
		}
		if !app.toast.active {
			t.Fatal("expected toast to be active after successful add")
		}
		assertContains(t, app.toast.text, "Added registry: new-reg")
		if app.toast.isErr {
			t.Fatal("expected success toast, not error")
		}
	})

	t.Run("error shows error toast", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registryAddDoneMsg{name: "bad-reg", err: fmt.Errorf("clone failed: timeout")})
		app = m.(App)

		if app.registryOpInProgress {
			t.Fatal("registryOpInProgress should be false after error")
		}
		if !app.toast.active {
			t.Fatal("expected toast to be active after failed add")
		}
		assertContains(t, app.toast.text, "Add failed")
		if !app.toast.isErr {
			t.Fatal("expected error toast")
		}
	})

	t.Run("empty repo shows warning toast", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registryAddDoneMsg{name: "empty-reg", empty: true})
		app = m.(App)

		assertContains(t, app.toast.text, "empty")
		if app.toast.isErr {
			t.Fatal("empty repo should show success toast with warning, not error")
		}
	})
}

// ---------------------------------------------------------------------------
// Registry Add Non-Syllago Message Tests
// ---------------------------------------------------------------------------

func TestRegistryAddNonSyllagoMsg(t *testing.T) {
	t.Run("opens redirect modal", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registryAddNonSyllagoMsg{
			name:      "native-reg",
			clonePath: "/tmp/test-clone",
			scan: catalog.NativeScanResult{
				Providers: []catalog.NativeProviderContent{
					{ProviderName: "Claude Code"},
				},
			},
		})
		app = m.(App)

		if app.registryOpInProgress {
			t.Fatal("registryOpInProgress should be false")
		}
		if !app.modal.active {
			t.Fatal("expected confirm modal to be active for non-syllago redirect")
		}
		if app.modal.purpose != modalNonSyllagoRedirect {
			t.Fatalf("expected modalNonSyllagoRedirect, got %d", app.modal.purpose)
		}
		if app.focus != focusModal {
			t.Fatal("expected focusModal for redirect dialog")
		}
		if app.pendingNonSyllagoClone != "/tmp/test-clone" {
			t.Fatal("pending clone path should be set")
		}
	})

	t.Run("confirm redirects to import", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registryAddNonSyllagoMsg{
			name:      "native-reg",
			clonePath: "/tmp/test-clone",
			scan: catalog.NativeScanResult{
				Providers: []catalog.NativeProviderContent{
					{ProviderName: "Cursor"},
				},
			},
		})
		app = m.(App)

		// Move cursor to Confirm (left) and press Enter
		m, _ = app.Update(tea.KeyMsg{Type: tea.KeyLeft})
		app = m.(App)
		m, _ = app.Update(keyEnter)
		app = m.(App)

		if app.screen != screenImport {
			t.Fatalf("expected screenImport after confirming redirect, got screen %d", app.screen)
		}
		if app.pendingNonSyllagoClone != "" {
			t.Fatal("pending clone path should be cleared after redirect")
		}
	})

	t.Run("y shortcut confirms redirect", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registryAddNonSyllagoMsg{
			name:      "native-reg",
			clonePath: "/tmp/test-clone2",
			scan: catalog.NativeScanResult{
				Providers: []catalog.NativeProviderContent{
					{ProviderName: "Cursor"},
				},
			},
		})
		app = m.(App)

		m, _ = app.Update(keyRune('y'))
		app = m.(App)

		if app.screen != screenImport {
			t.Fatalf("expected screenImport after 'y' shortcut, got screen %d", app.screen)
		}
	})

	t.Run("n shortcut dismisses redirect", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registryAddNonSyllagoMsg{
			name:      "native-reg",
			clonePath: "/tmp/test-clone3",
			scan: catalog.NativeScanResult{
				Providers: []catalog.NativeProviderContent{
					{ProviderName: "Cursor"},
				},
			},
		})
		app = m.(App)

		m, _ = app.Update(keyRune('n'))
		app = m.(App)

		if app.modal.active {
			t.Fatal("modal should be dismissed after 'n'")
		}
		assertScreen(t, app, screenRegistries)
	})
}

// ---------------------------------------------------------------------------
// Registry Remove Tests
// ---------------------------------------------------------------------------

func TestRegistryRemove(t *testing.T) {
	t.Run("r opens remove modal when entries exist", func(t *testing.T) {
		app := testAppWithRegistries(t)

		m, _ := app.Update(keyRune('r'))
		app = m.(App)

		if !app.modal.active {
			t.Fatal("expected confirm modal to open on 'r'")
		}
		if app.modal.purpose != modalRegistryRemove {
			t.Fatalf("expected modalRegistryRemove, got %d", app.modal.purpose)
		}
		assertContains(t, app.modal.body, "test-registry")
	})

	t.Run("r does nothing when no entries", func(t *testing.T) {
		app := navigateToRegistries(t) // no registries configured
		m, _ := app.Update(keyRune('r'))
		app = m.(App)

		if app.modal.active {
			t.Fatal("modal should not open when there are no registry entries")
		}
	})

	t.Run("r blocked during registry op", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(keyRune('r'))
		app = m.(App)

		if app.modal.active {
			t.Fatal("modal should not open when registryOpInProgress is true")
		}
	})

	t.Run("remove done success shows toast", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registryRemoveDoneMsg{name: "test-registry"})
		app = m.(App)

		if app.registryOpInProgress {
			t.Fatal("registryOpInProgress should be false after remove done")
		}
		if !app.toast.active {
			t.Fatal("expected toast after successful remove")
		}
		assertContains(t, app.toast.text, "Removed registry: test-registry")
	})

	t.Run("remove done error shows error toast", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registryRemoveDoneMsg{name: "test-registry", err: fmt.Errorf("permission denied")})
		app = m.(App)

		if !app.toast.isErr {
			t.Fatal("expected error toast on failed remove")
		}
		assertContains(t, app.toast.text, "Remove failed")
	})

	t.Run("cursor clamps after last entry removed", func(t *testing.T) {
		app := testAppWithRegistries(t)
		// Move cursor to the second entry (Right in 2-col grid)
		m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRight})
		app = m.(App)
		if app.cardCursor != 1 {
			t.Fatalf("expected cursor at 1, got %d", app.cardCursor)
		}

		// Simulate removing the entry — after rebuild, only 1 entry remains
		// We test cursor clamping by directly manipulating state
		app.registryOpInProgress = true
		app.registries.entries = app.registries.entries[:1]
		m, _ = app.Update(registryRemoveDoneMsg{name: "other-registry"})
		app = m.(App)

		// Cursor should clamp to the new last valid index
		if app.cardCursor > len(app.registries.entries)-1 && len(app.registries.entries) > 0 {
			t.Fatalf("cursor %d exceeds entries count %d", app.cardCursor, len(app.registries.entries))
		}
	})
}

// ---------------------------------------------------------------------------
// Registry Sync Tests
// ---------------------------------------------------------------------------

func TestRegistrySync(t *testing.T) {
	t.Run("s starts sync when entries exist", func(t *testing.T) {
		app := testAppWithRegistries(t)

		m, cmd := app.Update(keyRune('s'))
		app = m.(App)

		if !app.registryOpInProgress {
			t.Fatal("expected registryOpInProgress after pressing 's'")
		}
		if cmd == nil {
			t.Fatal("expected a command to be returned for async sync")
		}
		if !app.toast.active {
			t.Fatal("expected progress toast during sync")
		}
		assertContains(t, app.toast.text, "Syncing")
	})

	t.Run("s does nothing when no entries", func(t *testing.T) {
		app := navigateToRegistries(t) // empty
		m, cmd := app.Update(keyRune('s'))
		app = m.(App)

		if app.registryOpInProgress {
			t.Fatal("should not start sync with no entries")
		}
		if cmd != nil {
			t.Fatal("should not return command with no entries")
		}
	})

	t.Run("s blocked during registry op", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, cmd := app.Update(keyRune('s'))
		app = m.(App)

		if cmd != nil {
			t.Fatal("should not start another sync when op is in progress")
		}
	})

	t.Run("sync done success shows toast", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registrySyncDoneMsg{name: "test-registry"})
		app = m.(App)

		if app.registryOpInProgress {
			t.Fatal("registryOpInProgress should be false after sync done")
		}
		assertContains(t, app.toast.text, "Synced: test-registry")
	})

	t.Run("sync done error shows error toast", func(t *testing.T) {
		app := testAppWithRegistries(t)
		app.registryOpInProgress = true

		m, _ := app.Update(registrySyncDoneMsg{name: "test-registry", err: fmt.Errorf("network error")})
		app = m.(App)

		if !app.toast.isErr {
			t.Fatal("expected error toast on failed sync")
		}
		assertContains(t, app.toast.text, "Sync failed")
	})
}

// ---------------------------------------------------------------------------
// registryOpInProgress Guard Tests
// ---------------------------------------------------------------------------

func TestRegistryOpInProgressBlocksAllActions(t *testing.T) {
	app := testAppWithRegistries(t)
	app.registryOpInProgress = true

	tests := []struct {
		name string
		key  rune
	}{
		{"add blocked", 'a'},
		{"remove blocked", 'r'},
		{"sync blocked", 's'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cmd := app.Update(keyRune(tt.key))
			result := m.(App)

			if result.registryAddModal.active {
				t.Fatal("add modal should not open during active operation")
			}
			if result.modal.active {
				t.Fatal("confirm modal should not open during active operation")
			}
			// Sync returns a cmd, but not when blocked
			if tt.key == 's' && cmd != nil {
				t.Fatal("sync should not start during active operation")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Registry Card Navigation Tests
// ---------------------------------------------------------------------------

func TestRegistryCardNavigation(t *testing.T) {
	app := testAppWithRegistries(t)

	t.Run("initial cursor at 0", func(t *testing.T) {
		if app.cardCursor != 0 {
			t.Fatalf("expected cursor at 0, got %d", app.cardCursor)
		}
	})

	t.Run("right moves cursor in 2-col grid", func(t *testing.T) {
		// With 2 entries in a 2-column grid, Right moves to the next card
		m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRight})
		moved := m.(App)
		if moved.cardCursor != 1 {
			t.Fatalf("expected cursor at 1 after right, got %d", moved.cardCursor)
		}
	})

	t.Run("enter drills into registry items", func(t *testing.T) {
		m, _ := app.Update(keyEnter)
		drilled := m.(App)
		assertScreen(t, drilled, screenItems)
		if drilled.items.sourceRegistry != "test-registry" {
			t.Fatalf("expected sourceRegistry 'test-registry', got %q", drilled.items.sourceRegistry)
		}
	})
}

// ---------------------------------------------------------------------------
// Help Text Context Tests
// ---------------------------------------------------------------------------

func TestRegistryHelpText(t *testing.T) {
	t.Run("registries page shows registry-specific help", func(t *testing.T) {
		app := testAppWithRegistries(t)
		help := app.registries.helpText()
		assertContains(t, help, "add registry")
		assertContains(t, help, "remove registry")
		assertContains(t, help, "sync registry")
	})

	t.Run("items from registry do not show remove", func(t *testing.T) {
		app := testAppWithRegistries(t)
		// Drill into registry items
		m, _ := app.Update(keyEnter)
		app = m.(App)
		assertScreen(t, app, screenItems)

		help := app.items.helpText()
		// Registry items should NOT show remove (they're not library items)
		assertNotContains(t, help, "r remove")
		// But should show add and create loadout
		assertContains(t, help, "a add")
		assertContains(t, help, "l create loadout")
	})

	t.Run("items from content type show typed help", func(t *testing.T) {
		app := testApp(t)
		// Navigate to Skills items (cursor starts at Skills)
		m, _ := app.Update(keyEnter)
		app = m.(App)
		assertScreen(t, app, screenItems)

		help := app.items.helpText()
		assertContains(t, help, "add skill")
	})

	t.Run("library items show remove when removable items exist", func(t *testing.T) {
		app := navigateToLibraryItems(t)
		// The test catalog's local-skill has Library=true but Source is unset.
		// In production, the scanner sets Source="global". Patch it for this test.
		for i := range app.items.items {
			if app.items.items[i].Library {
				app.items.items[i].Source = "global"
			}
		}
		help := app.items.helpText()
		assertContains(t, help, "r remove")
	})
}

// ---------------------------------------------------------------------------
// Registry View Rendering Tests
// ---------------------------------------------------------------------------

func TestRegistryViewEmpty(t *testing.T) {
	app := navigateToRegistries(t)
	view := app.View()
	assertContains(t, view, "No registries configured")
	assertContains(t, view, "Press a to add")
}

func TestRegistryViewShowsCards(t *testing.T) {
	app := testAppWithRegistries(t)
	view := app.View()
	assertContains(t, view, "test-registry")
	assertContains(t, view, "other-registry")
}

// ---------------------------------------------------------------------------
// Registry Add Modal View Rendering
// ---------------------------------------------------------------------------

func TestRegistryAddModalView(t *testing.T) {
	app := navigateToRegistries(t)
	m, _ := app.Update(keyRune('a'))
	app = m.(App)

	view := app.registryAddModal.View()
	assertContains(t, view, "Add Registry")
	assertContains(t, view, "URL")
	assertContains(t, view, "Name")
	assertContains(t, view, "tab switch field")
}

// ---------------------------------------------------------------------------
// Item Remove from Items List Tests
// ---------------------------------------------------------------------------

func TestItemRemoveFromItemsList(t *testing.T) {
	t.Run("r opens remove modal for library items", func(t *testing.T) {
		app := navigateToLibraryItems(t)

		// The test catalog's local-skill has Library=true but Source is unset.
		// In production, the scanner sets Source="global". Patch it for this test.
		for i := range app.items.items {
			if app.items.items[i].Library {
				app.items.items[i].Source = "global"
				app.items.cursor = i
				break
			}
		}

		m, _ := app.Update(keyRune('r'))
		app = m.(App)

		if !app.modal.active {
			t.Fatal("expected confirm modal for library item remove")
		}
		if app.modal.purpose != modalItemRemove {
			t.Fatalf("expected modalItemRemove, got %d", app.modal.purpose)
		}
	})

	t.Run("r does nothing for non-removable items", func(t *testing.T) {
		app := testApp(t)
		// Navigate to Skills items (cursor starts at Skills, these are repo items not library)
		m, _ := app.Update(keyEnter)
		app = m.(App)
		assertScreen(t, app, screenItems)

		m, _ = app.Update(keyRune('r'))
		app = m.(App)

		if app.modal.active {
			t.Fatal("modal should not open for non-removable items")
		}
	})
}

// ---------------------------------------------------------------------------
// isRemovable Tests
// ---------------------------------------------------------------------------

func TestIsRemovable(t *testing.T) {
	tests := []struct {
		name     string
		item     catalog.ContentItem
		expected bool
	}{
		{
			name:     "library global item is removable",
			item:     catalog.ContentItem{Source: "global", Library: true},
			expected: true,
		},
		{
			name:     "non-library item is not removable",
			item:     catalog.ContentItem{Source: "global", Library: false},
			expected: false,
		},
		{
			name:     "registry item is not removable",
			item:     catalog.ContentItem{Source: "registry:test", Library: false},
			expected: false,
		},
		{
			name:     "empty source is not removable",
			item:     catalog.ContentItem{Source: "", Library: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRemovable(tt.item)
			if got != tt.expected {
				t.Fatalf("isRemovable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// contentTypeSingular Tests
// ---------------------------------------------------------------------------

func TestContentTypeSingular(t *testing.T) {
	tests := []struct {
		ct   catalog.ContentType
		want string
	}{
		{catalog.Skills, "skill"},
		{catalog.Agents, "agent"},
		{catalog.Rules, "rule"},
		{catalog.Hooks, "hook"},
		{catalog.Commands, "command"},
		{catalog.MCP, "mcp config"},
		{catalog.Loadouts, "loadout"},
	}

	for _, tt := range tests {
		t.Run(string(tt.ct), func(t *testing.T) {
			got := contentTypeSingular(tt.ct)
			if got != tt.want {
				t.Fatalf("contentTypeSingular(%q) = %q, want %q", tt.ct, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Registry Search Integration
// ---------------------------------------------------------------------------

func TestRegistrySearchIntegration(t *testing.T) {
	app := testAppWithRegistries(t)

	// Activate search
	m, _ := app.Update(keyRune('/'))
	app = m.(App)
	if !app.search.active {
		t.Fatal("expected search to activate")
	}

	// Type query matching only one registry
	for _, r := range "other" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}

	if len(app.registries.entries) != 1 {
		t.Fatalf("expected 1 filtered entry, got %d", len(app.registries.entries))
	}
	if app.registries.entries[0].name != "other-registry" {
		t.Fatalf("expected 'other-registry', got %q", app.registries.entries[0].name)
	}

	// Esc resets filter
	m, _ = app.Update(keyEsc)
	app = m.(App)
	if len(app.registries.entries) != 2 {
		t.Fatalf("expected 2 entries after reset, got %d", len(app.registries.entries))
	}
}
