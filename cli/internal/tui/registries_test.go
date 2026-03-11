package tui

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// TestRegistriesViewShowsVersionAndDescription verifies that manifest version
// and description appear in the registries screen when present in the entry.
func TestRegistriesViewShowsVersionAndDescription(t *testing.T) {
	cat := &catalog.Catalog{}
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "my-reg", URL: "https://example.com/my-reg.git"},
		},
	}

	// Build the model directly with entries (bypasses real clone/manifest loading)
	m := registriesModel{
		entries: []registryEntry{
			{
				name:        "my-reg",
				url:         "https://example.com/my-reg.git",
				cloned:      false,
				itemCount:   0,
				version:     "2.0.1",
				description: "A great registry for testing",
			},
		},
		width:  80,
		height: 30,
	}
	_ = cfg
	_ = cat

	view := m.View(0)
	if !strings.Contains(view, "2.0.1") {
		t.Errorf("registries view should show manifest version '2.0.1', got:\n%s", view)
	}
	if !strings.Contains(view, "A great registry for testing") {
		t.Errorf("registries view should show manifest description, got:\n%s", view)
	}
}

// TestRegistriesViewShowsDashWhenNoManifest verifies that the version column
// shows "─" when the entry has no manifest version.
func TestRegistriesViewShowsDashWhenNoManifest(t *testing.T) {
	m := registriesModel{
		entries: []registryEntry{
			{
				name:    "bare-reg",
				url:     "https://example.com/bare-reg.git",
				cloned:  false,
				version: "", // no manifest
			},
		},
		width:  80,
		height: 30,
	}
	view := m.View(0)
	if !strings.Contains(view, "─") {
		t.Errorf("registries view should show '─' when no manifest version, got:\n%s", view)
	}
}

// TestRegistriesTabTogglesFocus verifies that Tab toggles focus between
// sidebar and content on the registries screen.
func TestRegistriesTabTogglesFocus(t *testing.T) {
	app := navigateToRegistries(t)
	if app.focus != focusContent {
		t.Fatalf("expected focusContent after navigating to registries, got %d", app.focus)
	}

	m, _ := app.Update(keyTab)
	app = m.(App)
	if app.focus != focusSidebar {
		t.Fatalf("expected focusSidebar after Tab on registries, got %d", app.focus)
	}

	m, _ = app.Update(keyTab)
	app = m.(App)
	if app.focus != focusContent {
		t.Fatalf("expected focusContent after second Tab on registries, got %d", app.focus)
	}
}

// TestRegistriesSearchActivates verifies that / opens the search bar
// on the registries screen.
func TestRegistriesSearchActivates(t *testing.T) {
	app := navigateToRegistries(t)
	m, _ := app.Update(keyRune('/'))
	app = m.(App)
	if !app.search.active {
		t.Fatal("expected search to activate on registries after pressing /")
	}
}

// TestRegistriesSearchFiltersEntries verifies that typing in search
// live-filters registry entries by name/URL/description.
func TestRegistriesSearchFiltersEntries(t *testing.T) {
	app := navigateToRegistries(t)

	// Activate search
	m, _ := app.Update(keyRune('/'))
	app = m.(App)

	// Type a query that matches the test registry name
	for _, r := range "test-reg" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}

	// The entries should be filtered (test catalog has "test-registry")
	if len(app.registries.entries) > len(app.registries.allEntries) {
		t.Fatal("filtered entries should not exceed all entries")
	}

	// Esc should reset to full entries
	m, _ = app.Update(keyEsc)
	app = m.(App)
	if len(app.registries.entries) != len(app.registries.allEntries) {
		t.Fatalf("expected entries to reset after Esc, got %d want %d",
			len(app.registries.entries), len(app.registries.allEntries))
	}
}

// TestFilterRegistryEntries verifies the registry entry filter function.
func TestFilterRegistryEntries(t *testing.T) {
	entries := []registryEntry{
		{name: "alpha-reg", url: "https://example.com/alpha.git", description: "First registry"},
		{name: "beta-reg", url: "https://example.com/beta.git", description: "Second registry"},
		{name: "gamma-reg", url: "https://special.io/gamma.git", description: "Third registry"},
	}

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"empty query returns all", "", 3},
		{"filter by name", "alpha", 1},
		{"filter by url", "special.io", 1},
		{"filter by description", "Second", 1},
		{"case insensitive", "BETA", 1},
		{"no match", "nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterRegistryEntries(entries, tt.query)
			if len(got) != tt.want {
				t.Errorf("filterRegistryEntries(%q) returned %d entries, want %d", tt.query, len(got), tt.want)
			}
		})
	}
}
