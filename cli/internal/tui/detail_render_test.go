package tui

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

func TestRenderContentSplitHasSeparator(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{
			Name: "test-tool",
			Type: catalog.Prompts,
		},
		width:  60,
		height: 24,
	}
	pinned, _ := m.renderContentSplit()
	// The separator (─ repeated) must appear in the pinned section
	if !strings.Contains(pinned, "─") {
		t.Error("renderContentSplit pinned section should contain a horizontal separator (─)")
	}
}

func TestRenderContentSplitMetadataInPinned(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{
			Name: "test-tool",
			Type: catalog.Prompts,
		},
		width:  60,
		height: 24,
	}
	pinned, _ := m.renderContentSplit()
	// Type label must appear in pinned breadcrumb (Home > Prompts > test-tool)
	if !strings.Contains(pinned, "Prompts") {
		t.Error("renderContentSplit pinned section should contain type label in breadcrumb (e.g. 'Prompts')")
	}
}

// TestRenderTabBarHasZoneMarks verifies that renderTabBar() wraps each tab
// entry with zone.Mark(), which embeds ANSI escape sequences in the output.
func TestRenderTabBarHasZoneMarks(t *testing.T) {
	m := detailModel{
		item:      catalog.ContentItem{Name: "test", Type: catalog.Prompts},
		activeTab: tabOverview,
	}
	tabBar := m.renderTabBar()
	// zone.Mark() wraps content with ANSI escape sequences (\x1b[NNNNz...\x1b[NNNNz).
	// NO_COLOR suppresses lipgloss styling but not bubblezone markers.
	if !strings.Contains(tabBar, "\x1b[") {
		t.Error("renderTabBar() should contain ANSI escape sequences from zone.Mark() calls")
	}
}

func TestRenderInstallTabHasActionButtons(t *testing.T) {
	m := detailModel{
		item:      catalog.ContentItem{Name: "test-tool", Type: catalog.Prompts},
		activeTab: tabInstall,
		width:     60,
		height:    24,
	}
	body := m.renderInstallTab()
	// zone.Mark() embeds ANSI escape sequences in output, not the zone ID string.
	// Verify button labels are visible and that zone marks (ANSI escapes) are present.
	if !strings.Contains(body, "[i]nstall") {
		t.Error("renderInstallTab() should contain '[i]nstall' action button label")
	}
	if !strings.Contains(body, "[u]ninstall") {
		t.Error("renderInstallTab() should contain '[u]ninstall' action button label")
	}
	if !strings.Contains(body, "[c]opy") {
		t.Error("renderInstallTab() should contain '[c]opy' action button label")
	}
	if !strings.Contains(body, "[s]ave") {
		t.Error("renderInstallTab() should contain '[s]ave' action button label")
	}
	// ANSI zone marks must be present (zone.Mark() embeds \x1b[ escape sequences)
	if !strings.Contains(body, "\x1b[") {
		t.Error("renderInstallTab() should contain ANSI escape sequences from zone.Mark() calls on action buttons")
	}
}
func TestMessagePrefixes(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		isError    bool
		wantPrefix string
	}{
		{"error message has Error: prefix", "installation failed", true, "Error:"},
		{"success message has Done: prefix", "installed successfully", false, "Done:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := testApp(t)
			app = pressN(app, keyEnter, 1) // → items
			app = pressN(app, keyEnter, 1) // → detail

			app.detail.message = tt.message
			app.detail.messageIsErr = tt.isError

			view := app.detail.View()
			assertContains(t, view, tt.wantPrefix+" "+tt.message)
		})
	}
}
