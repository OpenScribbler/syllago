package tui

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/config"
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

	view := m.View()
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
	view := m.View()
	if !strings.Contains(view, "─") {
		t.Errorf("registries view should show '─' when no manifest version, got:\n%s", view)
	}
}
