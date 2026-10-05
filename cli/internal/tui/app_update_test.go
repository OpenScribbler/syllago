package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// TestApp_HandleLibraryAddMsg_ShowsToast verifies that when the App receives
// a libraryAddMsg it returns a non-nil cmd (toast push or async operation).
func TestApp_HandleLibraryAddMsg_ShowsToast(t *testing.T) {
	t.Parallel()
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{
				Name:     "registry-skill",
				Type:     catalog.Skills,
				Registry: "test-registry",
				Library:  false,
				Files:    []string{"SKILL.md"},
			},
		},
	}
	app := NewApp(&moat.ScanResult{Catalog: cat, Config: testConfig()}, testProviders(), "0.0.0-test", false, false, "", "")
	item := &cat.Items[0]
	_, cmd := app.Update(libraryAddMsg{item: item})
	if cmd == nil {
		t.Error("handling libraryAddMsg must return a non-nil cmd (toast or operation)")
	}
}
