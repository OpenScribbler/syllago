package tui

import (
	"fmt"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

func TestItemsNavigationUpDown(t *testing.T) {
	app := testApp(t)
	// Enter Skills (cursor=0, first type)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	nItems := len(app.items.items)
	if nItems < 2 {
		t.Fatalf("expected at least 2 skill items, got %d", nItems)
	}

	// Move down
	app = pressN(app, keyDown, 1)
	if app.items.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", app.items.cursor)
	}

	// Bounds: can't go past last item
	app = pressN(app, keyDown, nItems+5)
	if app.items.cursor != nItems-1 {
		t.Fatalf("expected cursor clamped at %d, got %d", nItems-1, app.items.cursor)
	}

	// Navigate back up
	app = pressN(app, keyUp, nItems+5)
	if app.items.cursor != 0 {
		t.Fatalf("expected cursor 0 after up, got %d", app.items.cursor)
	}
}

func TestItemsEnterOpensDetail(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyEnter) // → items (Skills)
	app = m.(App)
	assertScreen(t, app, screenItems)

	selectedName := app.items.selectedItem().Name
	m, _ = app.Update(keyEnter) // → detail
	app = m.(App)
	assertScreen(t, app, screenDetail)

	if app.detail.item.Name != selectedName {
		t.Fatalf("expected detail for %q, got %q", selectedName, app.detail.item.Name)
	}
}

func TestItemsBackGoesToCategory(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyEnter) // → items (focus=content)
	app = m.(App)
	assertScreen(t, app, screenItems)

	m, _ = app.Update(keyEsc) // Single Esc: back to category screen
	app = m.(App)
	if app.screen != screenCategory {
		t.Fatalf("expected screen=screenCategory after Esc from items, got %d", app.screen)
	}
	if app.focus != focusSidebar {
		t.Fatalf("expected focus=focusSidebar after Esc from items, got %d", app.focus)
	}
}

func TestQFromItemsNavigatesBack(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyEnter) // → items (focus=content)
	app = m.(App)
	assertScreen(t, app, screenItems)

	// q from content focus: synthesizes Esc → goes back to category screen
	m, _ = app.Update(keyRune('q'))
	app = m.(App)
	if app.screen != screenCategory {
		t.Fatalf("expected screen=screenCategory after q from items, got %d", app.screen)
	}
	// q from category/sidebar should quit
	_, cmd := app.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("q from sidebar should produce quit command")
	}
}

func TestQFromDetailNavigatesBack(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('q'))
	app = m.(App)
	assertScreen(t, app, screenItems)
}

func TestItemsEmptyList(t *testing.T) {
	app := testApp(t)
	// Navigate to a type with no items (Commands has 1 provider-specific item,
	// but let's construct an empty case manually)
	emptyItems := newItemsModel(catalog.Skills, nil, nil, "/tmp")
	emptyItems.width = 80
	emptyItems.height = 30
	app.items = emptyItems
	app.screen = screenItems

	view := app.View()
	assertContains(t, view, "No items found")

	// Enter on empty list shouldn't crash
	m, _ := app.Update(keyEnter)
	app = m.(App)
	// Should still be on items screen (enter does nothing with empty list)
	assertScreen(t, app, screenItems)
}

func TestItemsScrollIndicators(t *testing.T) {
	// Create a large number of items to force scrolling
	var items []catalog.ContentItem
	for i := 0; i < 50; i++ {
		items = append(items, catalog.ContentItem{
			Name:        "skill-" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			Description: "Description for item",
			Type:        catalog.Skills,
			Path:        "/tmp/items/" + string(rune(i)),
		})
	}

	app := testApp(t)
	app.items = newItemsModel(catalog.Skills, items, nil, "/tmp")
	app.items.width = 80
	app.items.height = 30
	app.screen = screenItems

	// At the top, no "above" indicator but should have "below" indicator
	view := app.View()
	assertNotContains(t, view, "more above")
	assertContains(t, view, "more below")

	// Navigate down past the visible area
	app = pressN(app, keyDown, 40)
	view = app.View()
	assertContains(t, view, "more above")
}

func TestItemsScrollShowsCounts(t *testing.T) {
	items := make([]catalog.ContentItem, 50)
	for i := range items {
		items[i] = catalog.ContentItem{Name: fmt.Sprintf("item-%02d", i), Type: catalog.Skills}
	}
	m := newItemsModel(catalog.Skills, items, nil, "/tmp")
	m.width = 80
	m.height = 10 // small height forces scrolling

	// Navigate to bottom
	for i := 0; i < 40; i++ {
		m, _ = m.Update(keyDown)
	}

	view := m.View()
	assertContains(t, view, "more above")
	// Should show a number, not just generic text
	assertNotContains(t, view, "(more items above)")
}

func TestItemsProviderColumn(t *testing.T) {
	app := testApp(t)
	// Navigate to Rules (provider-specific type)
	nTypes := len(catalog.AllContentTypes())
	// Rules is one of the types — find its index
	for i, ct := range catalog.AllContentTypes() {
		if ct == catalog.Rules {
			app = pressN(app, keyDown, i)
			break
		}
		_ = nTypes // suppress unused
	}
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	if len(app.items.items) == 0 {
		t.Fatal("expected at least one rule item")
	}

	view := app.View()
	// Provider column header should appear
	assertContains(t, view, "Provider")
	// Provider name should appear for the rule item
	assertContains(t, view, "Claude Code")
}

func TestItemsLocalPrefix(t *testing.T) {
	app := testApp(t)
	// Navigate to My Tools
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes) // My Tools
	m, _ := app.Update(keyEnter)
	app = m.(App)

	if len(app.items.items) == 0 {
		t.Fatal("expected at least one local item in My Tools")
	}

	view := app.View()
	assertContains(t, view, "[LOCAL]")
}

func TestItemsSearchResultsTypeTag(t *testing.T) {
	app := testApp(t)
	// Simulate search results view
	items := []catalog.ContentItem{
		{Name: "a-skill", Type: catalog.Skills, Description: "Skill item"},
		{Name: "a-rule", Type: catalog.Rules, Description: "Rule item", Provider: "claude-code"},
	}
	app.items = newItemsModel(catalog.SearchResults, items, app.providers, app.catalog.RepoRoot)
	app.items.width = 80
	app.items.height = 30
	app.screen = screenItems

	view := app.View()
	assertContains(t, view, "Search Results")
	// Type tags should be shown for mixed-type views
	assertContains(t, view, catalog.Skills.Label())
}

func TestItemsMyToolsTypeTag(t *testing.T) {
	app := testApp(t)
	// Navigate to My Tools
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes)
	m, _ := app.Update(keyEnter)
	app = m.(App)

	if len(app.items.items) == 0 {
		t.Fatal("expected at least one My Tools item")
	}

	view := app.View()
	assertContains(t, view, "My Tools")
	// Type tags should appear
	assertContains(t, view, catalog.Skills.Label())
}

func TestItemsHomeEnd(t *testing.T) {
	var items []catalog.ContentItem
	for i := 0; i < 20; i++ {
		items = append(items, catalog.ContentItem{
			Name: fmt.Sprintf("item-%02d", i),
			Type: catalog.Skills,
			Path: fmt.Sprintf("/tmp/items/%d", i),
		})
	}
	m := newItemsModel(catalog.Skills, items, nil, "/tmp")
	m.width = 80
	m.height = 30

	// End jumps to last
	m, _ = m.Update(keyRune('G'))
	if m.cursor != 19 {
		t.Fatalf("expected cursor 19 after End/G, got %d", m.cursor)
	}

	// Home jumps to first
	m, _ = m.Update(keyRune('g'))
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0 after Home/g, got %d", m.cursor)
	}
}

func TestTableHeaderStyleIsBold(t *testing.T) {
	if !tableHeaderStyle.GetBold() {
		t.Fatal("tableHeaderStyle should be bold")
	}
}

func TestMyToolsEmptyGuidance(t *testing.T) {
	m := newItemsModel(catalog.MyTools, nil, nil, "/tmp")
	m.width = 80
	m.height = 30

	view := m.View()
	assertContains(t, view, "Import")
	assertContains(t, view, "nesco add")
}

func TestItemsCursorIsASCII(t *testing.T) {
	app := testApp(t)
	app = pressN(app, keyEnter, 1) // → items
	view := app.items.View()

	assertContains(t, view, " > ")
	assertNotContains(t, view, "▸")
}

func TestItemsCursorPreserved(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyEnter) // → items (Skills)
	app = m.(App)
	assertScreen(t, app, screenItems)

	if len(app.items.items) < 2 {
		t.Skip("need at least 2 items to test cursor preservation")
	}

	// Move cursor to second item
	app = pressN(app, keyDown, 1)
	if app.items.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", app.items.cursor)
	}

	// Enter detail
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenDetail)

	// Go back to items
	m, _ = app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenItems)

	// Cursor should be preserved at position 1
	if app.items.cursor != 1 {
		t.Fatalf("expected cursor preserved at 1, got %d", app.items.cursor)
	}
}

func TestItemsTruncation(t *testing.T) {
	longName := "this-is-a-very-long-skill-name-that-should-be-truncated-in-the-view"
	longDesc := "This is a very long description that exceeds the available width and should be truncated with an ellipsis at the end"
	items := []catalog.ContentItem{
		{Name: longName, Description: longDesc, Type: catalog.Skills, Path: "/tmp/long"},
	}

	app := testApp(t)
	app.items = newItemsModel(catalog.Skills, items, nil, "/tmp")
	app.items.width = 60 // narrow terminal
	app.items.height = 30
	app.screen = screenItems

	view := app.View()
	// The view should not be wider than what's reasonable
	// (truncation with "..." should happen)
	for _, line := range splitLines(view) {
		if len(line) > 80 { // some tolerance for ANSI codes in non-NO_COLOR envs
			// Just verify truncation happened — the "..." suffix
			if len(longName) > 60 {
				assertContains(t, view, "...")
			}
			break
		}
	}
}

// splitLines splits a string into lines, handling both \n and \r\n.
func splitLines(s string) []string {
	var lines []string
	for _, line := range split(s, "\n") {
		lines = append(lines, line)
	}
	return lines
}

func split(s, sep string) []string {
	var result []string
	for {
		i := indexOf(s, sep)
		if i < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:i])
		s = s[i+len(sep):]
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
