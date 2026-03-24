package tui_v1

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestSearchActivateCategory(t *testing.T) {
	app := testApp(t)
	assertScreen(t, app, screenCategory)

	m, _ := app.Update(keyRune('/'))
	app = m.(App)

	if !app.search.active {
		t.Fatal("expected search to be active after / from category")
	}
}

func TestSearchActivateItems(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyEnter) // → items
	app = m.(App)
	assertScreen(t, app, screenItems)

	m, _ = app.Update(keyRune('/'))
	app = m.(App)

	if !app.search.active {
		t.Fatal("expected search to be active after / from items")
	}
}

func TestSearchBlockedImport(t *testing.T) {
	app := testApp(t)
	// Navigate to import screen
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+3)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenImport)

	m, _ = app.Update(keyRune('/'))
	app = m.(App)
	if app.search.active {
		t.Fatal("search should NOT activate from import screen")
	}
}

func TestSearchBlockedUpdate(t *testing.T) {
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+4)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenUpdate)

	m, _ = app.Update(keyRune('/'))
	app = m.(App)
	if app.search.active {
		t.Fatal("search should NOT activate from update screen")
	}
}

func TestSearchBlockedSettings(t *testing.T) {
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+5)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenSettings)

	m, _ = app.Update(keyRune('/'))
	app = m.(App)
	if app.search.active {
		t.Fatal("search should NOT activate from settings screen")
	}
}

// TestSearchBlockedDetailTextInput was removed — confirmAction/actionEnvValue
// no longer exist on detailModel. Text input is now handled by the centralized
// modal system, and HasTextInput() always returns false on detail.

func TestSearchTypeQuery(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyRune('/'))
	app = m.(App)

	// Type some characters
	for _, r := range "test" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}

	if app.search.query() != "test" {
		t.Fatalf("expected query %q, got %q", "test", app.search.query())
	}
}

func TestSearchEnterFromCategory(t *testing.T) {
	app := testApp(t)

	// Activate search and type a query
	m, _ := app.Update(keyRune('/'))
	app = m.(App)
	for _, r := range "skill" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}

	// Enter should transition to SearchResults items
	m, _ = app.Update(keyEnter)
	app = m.(App)

	assertScreen(t, app, screenItems)
	if app.items.contentType != catalog.SearchResults {
		t.Fatalf("expected SearchResults, got %s", app.items.contentType)
	}
	// Search should be deactivated after enter.
	if app.search.active {
		t.Fatal("search should be deactivated after enter")
	}
}

func TestSearchEnterFromItems(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyEnter) // → items (Skills)
	app = m.(App)

	// Activate search and type
	m, _ = app.Update(keyRune('/'))
	app = m.(App)
	for _, r := range "alpha" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}

	// Enter should filter within the same type
	m, _ = app.Update(keyEnter)
	app = m.(App)

	assertScreen(t, app, screenItems)
	// Should still be Skills type (filtering within)
	if app.items.contentType != catalog.Skills {
		t.Fatalf("expected Skills type preserved, got %s", app.items.contentType)
	}
	// Should have filtered results
	for _, item := range app.items.items {
		if item.Type != catalog.Skills {
			t.Fatalf("filtered items should all be Skills, got %s", item.Type)
		}
	}
}

func TestSearchEscCancels(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyRune('/'))
	app = m.(App)

	// Type something
	for _, r := range "query" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}

	// Esc should cancel and clear
	m, _ = app.Update(keyEsc)
	app = m.(App)

	if app.search.active {
		t.Fatal("search should be deactivated after esc")
	}
	if app.search.query() != "" {
		t.Fatalf("expected empty query after esc, got %q", app.search.query())
	}
}

func TestSearchEscResetsItems(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyEnter) // → items (Skills)
	app = m.(App)
	origCount := len(app.items.items)

	// Search and filter
	m, _ = app.Update(keyRune('/'))
	app = m.(App)
	for _, r := range "alpha" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}
	m, _ = app.Update(keyEnter) // apply filter
	app = m.(App)

	filteredCount := len(app.items.items)
	if filteredCount >= origCount && origCount > 1 {
		t.Fatal("expected fewer items after filtering")
	}

	// Activate search again and esc to reset
	m, _ = app.Update(keyRune('/'))
	app = m.(App)
	m, _ = app.Update(keyEsc)
	app = m.(App)

	// Items should be reset to full list
	if len(app.items.items) != origCount {
		t.Fatalf("expected %d items after esc reset, got %d", origCount, len(app.items.items))
	}
}

func TestFilterItemsFunction(t *testing.T) {
	items := []catalog.ContentItem{
		{Name: "alpha-skill", Description: "First skill", Provider: "claude-code"},
		{Name: "beta-agent", Description: "An agent", Provider: "cursor"},
		{Name: "gamma-prompt", Description: "Alpha related prompt", Provider: ""},
	}

	// Match by name
	result := filterItems(items, "alpha")
	if len(result) != 2 { // "alpha-skill" by name + "gamma-prompt" by description
		t.Fatalf("expected 2 results for 'alpha', got %d", len(result))
	}

	// Match by provider
	result = filterItems(items, "cursor")
	if len(result) != 1 {
		t.Fatalf("expected 1 result for 'cursor', got %d", len(result))
	}
	if result[0].Name != "beta-agent" {
		t.Fatalf("expected beta-agent, got %s", result[0].Name)
	}

	// Case insensitive
	result = filterItems(items, "BETA")
	if len(result) != 1 {
		t.Fatalf("expected 1 result for 'BETA', got %d", len(result))
	}

	// Empty query returns all
	result = filterItems(items, "")
	if len(result) != 3 {
		t.Fatalf("expected 3 results for empty query, got %d", len(result))
	}

	// No match
	result = filterItems(items, "zzzzz")
	if len(result) != 0 {
		t.Fatalf("expected 0 results for 'zzzzz', got %d", len(result))
	}
}

func TestSearchHasLabel(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyRune('/'))
	app = m.(App)

	view := app.View()
	assertContains(t, view, "Search:")
}

func TestSearchLiveFilterItems(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyEnter) // → items (Skills)
	app = m.(App)
	origCount := len(app.items.items)

	// Activate search
	m, _ = app.Update(keyRune('/'))
	app = m.(App)

	// Type "alpha" — should filter live
	for _, r := range "alpha" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}

	// Items should be filtered without pressing Enter
	if len(app.items.items) >= origCount && origCount > 1 {
		t.Fatalf("expected fewer items during live search, got %d (was %d)", len(app.items.items), origCount)
	}
}

func TestSearchLiveFilterCategoryShowsCount(t *testing.T) {
	app := testApp(t)
	assertScreen(t, app, screenCategory)

	m, _ := app.Update(keyRune('/'))
	app = m.(App)

	for _, r := range "skill" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}

	// Match count should be visible
	view := app.View()
	assertContains(t, view, "matches)")
}

func TestSearchReplacesHelpBar(t *testing.T) {
	app := testApp(t)

	// Before search: help bar is visible (footer shows / search hint)
	view := app.View()
	assertContains(t, view, "/ search")

	// Activate search
	m, _ := app.Update(keyRune('/'))
	app = m.(App)

	view = app.View()
	assertContains(t, view, "Search:")
	// Help bar should be replaced, not both showing
	assertNotContains(t, view, "/ search")
}
