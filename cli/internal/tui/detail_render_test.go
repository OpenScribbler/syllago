package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
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

// TestRenderOverviewTabShowsRiskIndicators verifies that hook items with command
// hooks display a risk indicator in the Overview tab.
func TestRenderOverviewTabShowsRiskIndicators(t *testing.T) {
	dir := t.TempDir()
	hookJSON := `{"hooks":{"PostToolUse":[{"matcher":"Write","command":"echo hi"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := detailModel{
		item: catalog.ContentItem{
			Name:  "test-hook",
			Type:  catalog.Hooks,
			Path:  dir,
			Files: []string{"hook.json"},
		},
		width:  60,
		height: 24,
	}
	body := m.renderOverviewTab()
	if !strings.Contains(body, "Runs commands") {
		t.Errorf("Overview tab should show 'Runs commands' risk indicator, got:\n%s", body)
	}
}

// TestRenderOverviewTabNoRiskForPrompts verifies that prompt items (which have
// no risk signals) show nothing extra in the Overview tab.
func TestRenderOverviewTabNoRiskForPrompts(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{
			Name:  "safe-prompt",
			Type:  catalog.Prompts,
			Body:  "You are a helpful assistant.",
			Files: []string{"PROMPT.md"},
		},
		width:  60,
		height: 24,
	}
	body := m.renderOverviewTab()
	if strings.Contains(body, "⚠") {
		t.Errorf("Overview tab should not show risk indicators for Prompts, got:\n%s", body)
	}
}

func TestRenderOverrideInfoShown(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{
			Name:  "my-skill",
			Type:  catalog.Skills,
			Local: true,
		},
		overrides: []catalog.ContentItem{
			{
				Name:     "my-skill",
				Type:     catalog.Skills,
				Registry: "community",
			},
		},
		width:  60,
		height: 24,
	}
	pinned, _ := m.renderContentSplit()
	if !strings.Contains(pinned, "Overrides") {
		t.Errorf("pinned section should contain 'Overrides' when overrides exist, got:\n%s", pinned)
	}
	if !strings.Contains(pinned, "community") {
		t.Errorf("pinned section should mention the overridden registry name 'community', got:\n%s", pinned)
	}
}

func TestRenderOverrideInfoBuiltIn(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{
			Name:  "my-skill",
			Type:  catalog.Skills,
			Local: true,
		},
		overrides: []catalog.ContentItem{
			{
				Name: "my-skill",
				Type: catalog.Skills,
				Meta: &metadata.Meta{Tags: []string{"builtin"}},
			},
		},
		width:  60,
		height: 24,
	}
	pinned, _ := m.renderContentSplit()
	if !strings.Contains(pinned, "built-in") {
		t.Errorf("pinned section should mention 'built-in' for builtin overrides, got:\n%s", pinned)
	}
}

func TestRenderOverrideInfoNotShownWhenEmpty(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{
			Name: "my-skill",
			Type: catalog.Skills,
		},
		overrides: nil,
		width:     60,
		height:    24,
	}
	pinned, _ := m.renderContentSplit()
	if strings.Contains(pinned, "Overrides") {
		t.Errorf("pinned section should NOT contain 'Overrides' when there are none, got:\n%s", pinned)
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
