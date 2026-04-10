package tui_v1

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestRenderContentSplitHasSeparator(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{
			Name: "test-tool",
			Type: catalog.Skills,
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
			Type: catalog.Skills,
		},
		width:  60,
		height: 24,
	}
	pinned, _ := m.renderContentSplit()
	// Type label must appear in pinned breadcrumb (Home > Skills > test-tool)
	if !strings.Contains(pinned, "Skills") {
		t.Error("renderContentSplit pinned section should contain type label in breadcrumb (e.g. 'Skills')")
	}
}

// TestRenderTabBarHasZoneMarks verifies that renderTabBar() wraps each tab
// entry with zone.Mark(), which embeds ANSI escape sequences in the output.
func TestRenderTabBarHasZoneMarks(t *testing.T) {
	m := detailModel{
		item:      catalog.ContentItem{Name: "test", Type: catalog.Skills},
		activeTab: tabFiles,
	}
	tabBar := m.renderTabBar()
	// zone.Mark() wraps content with ANSI escape sequences (\x1b[NNNNz...\x1b[NNNNz).
	// NO_COLOR suppresses lipgloss styling but not bubblezone markers.
	if !strings.Contains(tabBar, "\x1b[") {
		t.Error("renderTabBar() should contain ANSI escape sequences from zone.Mark() calls")
	}
}

func TestTabOrder_Hooks(t *testing.T) {
	m := detailModel{item: catalog.ContentItem{Type: catalog.Hooks}}
	tabs := m.tabOrder()
	if len(tabs) != 3 {
		t.Fatalf("hook items should have 3 tabs, got %d", len(tabs))
	}
	if tabs[1] != tabCompatibility {
		t.Error("second tab for hooks should be Compatibility")
	}
}

func TestTabOrder_NonHooks(t *testing.T) {
	m := detailModel{item: catalog.ContentItem{Type: catalog.Skills}}
	tabs := m.tabOrder()
	if len(tabs) != 2 {
		t.Fatalf("non-hook items should have 2 tabs, got %d", len(tabs))
	}
}

func TestRenderInstallTab_MCPConfig(t *testing.T) {
	m := detailModel{
		item:      catalog.ContentItem{Name: "test-mcp", Type: catalog.MCP},
		activeTab: tabInstall,
		mcpConfig: &installer.MCPConfig{
			Type:    "stdio",
			Command: "node",
			Args:    []string{"server.js", "--port=3000"},
			URL:     "https://mcp.example.com",
			Env:     map[string]string{"API_KEY": "placeholder"},
		},
		width:  60,
		height: 24,
	}
	got := stripANSI(m.renderInstallTab())
	if !strings.Contains(got, "Server Configuration") {
		t.Error("MCP install tab should show 'Server Configuration'")
	}
	if !strings.Contains(got, "stdio") {
		t.Error("should show server type")
	}
	if !strings.Contains(got, "node") {
		t.Error("should show command")
	}
	if !strings.Contains(got, "API_KEY") {
		t.Error("should show env var name")
	}
}

func TestRenderInstallTabHasActionButtons(t *testing.T) {
	m := detailModel{
		item:      catalog.ContentItem{Name: "test-tool", Type: catalog.Skills},
		activeTab: tabInstall,
		width:     60,
		height:    24,
	}
	body := m.renderInstallTab()
	stripped := stripANSI(body)
	// Verify styled button labels are visible
	if !strings.Contains(stripped, "Install") {
		t.Error("renderInstallTab() should contain 'Install' action button")
	}
	if !strings.Contains(stripped, "Uninstall") {
		t.Error("renderInstallTab() should contain 'Uninstall' action button")
	}
}

func TestRenderOverrideInfoShown(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{
			Name:    "my-skill",
			Type:    catalog.Skills,
			Library: true,
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
			Name:    "my-skill",
			Type:    catalog.Skills,
			Library: true,
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

// ---------------------------------------------------------------------------
// hookCompatForProvider tests
// ---------------------------------------------------------------------------

func TestHookCompatForProvider(t *testing.T) {
	providers := converter.HookProviders() // ["claude-code", "gemini-cli", "copilot-cli", "kiro"]

	compats := make([]converter.CompatResult, len(providers))
	for i, p := range providers {
		compats[i] = converter.CompatResult{
			Provider: p,
			Level:    converter.CompatLevel(i), // Full, Degraded, Broken, None
		}
	}

	tests := []struct {
		name      string
		compat    []converter.CompatResult
		slug      string
		wantNil   bool
		wantLevel converter.CompatLevel
	}{
		{"nil compat returns nil", nil, "claude-code", true, 0},
		{"found claude-code", compats, "claude-code", false, converter.CompatFull},
		{"found gemini-cli", compats, "gemini-cli", false, converter.CompatDegraded},
		{"found kiro", compats, "kiro", false, converter.CompatNone},
		{"unknown slug returns nil", compats, "unknown-provider", true, 0},
		{"empty slug returns nil", compats, "", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := detailModel{hookCompat: tt.compat}
			got := m.hookCompatForProvider(tt.slug)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if got.Level != tt.wantLevel {
				t.Errorf("level = %v, want %v", got.Level, tt.wantLevel)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// renderCompatibilityTab tests
// ---------------------------------------------------------------------------

func TestRenderCompatibilityTab_NonHookItem(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{Name: "test", Type: catalog.Skills},
	}
	got := m.renderCompatibilityTab()
	if !strings.Contains(got, "not available") {
		t.Error("non-hook item should show 'not available' message")
	}
}

func TestRenderCompatibilityTab_NilHookData(t *testing.T) {
	m := detailModel{
		item:     catalog.ContentItem{Name: "test", Type: catalog.Hooks},
		hookData: nil,
	}
	got := m.renderCompatibilityTab()
	if !strings.Contains(got, "not available") {
		t.Error("nil hookData should show 'not available' message")
	}
}

func TestRenderCompatibilityTab_WithCompat(t *testing.T) {
	m := detailModel{
		item:     catalog.ContentItem{Name: "my-hook", Type: catalog.Hooks},
		hookData: &converter.HookData{Event: "before_tool_execute"},
		hookCompat: []converter.CompatResult{
			{Provider: "claude-code", Level: converter.CompatFull, Notes: "native support"},
			{Provider: "gemini-cli", Level: converter.CompatDegraded, Notes: "no matcher"},
			{Provider: "copilot-cli", Level: converter.CompatBroken, Notes: "event mismatch"},
			{Provider: "kiro", Level: converter.CompatNone, Notes: "unsupported event"},
		},
	}
	got := stripANSI(m.renderCompatibilityTab())
	// Should contain provider names and levels
	if !strings.Contains(got, "Claude") {
		t.Error("should contain 'Claude' provider name")
	}
	if !strings.Contains(got, "Full") {
		t.Error("should contain 'Full' level label")
	}
	if !strings.Contains(got, "Degraded") {
		t.Error("should contain 'Degraded' level label")
	}
	if !strings.Contains(got, "Broken") {
		t.Error("should contain 'Broken' level label")
	}
	if !strings.Contains(got, "None") {
		t.Error("should contain 'None' level label")
	}
	if !strings.Contains(got, "native support") {
		t.Error("should contain notes text")
	}
}

func TestRenderCompatibilityTab_Warnings(t *testing.T) {
	m := detailModel{
		item:     catalog.ContentItem{Name: "my-hook", Type: catalog.Hooks},
		hookData: &converter.HookData{Event: "before_tool_execute"},
		hookCompat: []converter.CompatResult{
			{
				Provider: "claude-code",
				Level:    converter.CompatBroken,
				Notes:    "issues",
				Features: []converter.FeatureResult{
					{Present: true, Supported: false, Impact: converter.CompatBroken, Notes: "timeout not supported"},
				},
			},
		},
	}
	got := stripANSI(m.renderCompatibilityTab())
	if !strings.Contains(got, "Warnings") {
		t.Error("should contain 'Warnings' section when broken features exist")
	}
	if !strings.Contains(got, "timeout not supported") {
		t.Error("should contain feature warning notes")
	}
}

// ---------------------------------------------------------------------------
// renderSinglePanePreview tests (splitViewModel)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// primaryFileIndex tests
// ---------------------------------------------------------------------------

func TestPrimaryFileIndex(t *testing.T) {
	tests := []struct {
		name  string
		items []splitViewItem
		ct    catalog.ContentType
		want  int
	}{
		{
			"skill finds SKILL.md",
			[]splitViewItem{
				{Label: "README.md", IsDir: false},
				{Label: "SKILL.md", IsDir: false},
			},
			catalog.Skills,
			1,
		},
		{
			"agent finds first .md",
			[]splitViewItem{
				{Label: "config.json", IsDir: false},
				{Label: "agent.md", IsDir: false},
			},
			catalog.Agents,
			1,
		},
		{
			"hook finds .json",
			[]splitViewItem{
				{Label: "README.md", IsDir: false},
				{Label: "hook.json", IsDir: false},
			},
			catalog.Hooks,
			1,
		},
		{
			"hook finds .yaml",
			[]splitViewItem{
				{Label: "README.md", IsDir: false},
				{Label: "hook.yaml", IsDir: false},
			},
			catalog.Hooks,
			1,
		},
		{
			"mcp finds .json",
			[]splitViewItem{
				{Label: "README.md", IsDir: false},
				{Label: "server.json", IsDir: false},
			},
			catalog.MCP,
			1,
		},
		{
			"commands returns first non-disabled",
			[]splitViewItem{
				{Label: "header", Disabled: true},
				{Label: "run.sh", IsDir: false},
			},
			catalog.Commands,
			1,
		},
		{
			"loadout finds loadout.yaml",
			[]splitViewItem{
				{Label: "README.md", IsDir: false},
				{Label: "loadout.yaml", IsDir: false},
			},
			catalog.Loadouts,
			1,
		},
		{
			"skips disabled items",
			[]splitViewItem{
				{Label: "SKILL.md", Disabled: true},
				{Label: "other.md", IsDir: false},
			},
			catalog.Skills,
			1, // Falls through to first non-disabled for Skills since SKILL.md is disabled
		},
		{
			"skips directories",
			[]splitViewItem{
				{Label: "SKILL.md", IsDir: true},
				{Label: "SKILL.md", IsDir: false},
			},
			catalog.Skills,
			1,
		},
		{
			"empty items returns 0",
			[]splitViewItem{},
			catalog.Skills,
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := primaryFileIndex(tt.items, tt.ct)
			if got != tt.want {
				t.Errorf("primaryFileIndex() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// splitViewModel.SetItems tests
// ---------------------------------------------------------------------------

func TestSplitViewSetItems(t *testing.T) {
	m := splitViewModel{
		items:          []splitViewItem{{Label: "old.txt"}},
		cursor:         5,
		scrollOffset:   3,
		previewContent: "old content",
		previewScroll:  2,
		showingPreview: true,
		focusedPane:    panePreview,
	}
	newItems := []splitViewItem{{Label: "new1.txt"}, {Label: "new2.txt"}}
	m.SetItems(newItems)

	if len(m.items) != 2 {
		t.Errorf("items len = %d, want 2", len(m.items))
	}
	if m.cursor != 0 {
		t.Error("cursor should reset to 0")
	}
	if m.scrollOffset != 0 {
		t.Error("scrollOffset should reset to 0")
	}
	if m.previewContent != "" {
		t.Error("previewContent should be cleared")
	}
	if m.previewScroll != 0 {
		t.Error("previewScroll should reset to 0")
	}
	if m.showingPreview {
		t.Error("showingPreview should be false")
	}
	if m.focusedPane != paneList {
		t.Error("focusedPane should reset to paneList")
	}
}

func TestRenderSinglePanePreview_EmptyContent(t *testing.T) {
	m := splitViewModel{
		items: []splitViewItem{
			{Label: "README.md", Path: "/tmp/readme.md"},
		},
		cursor:         0,
		showingPreview: true,
		previewContent: "",
		width:          40,
		height:         20,
		zonePrefix:     "sv-test",
	}
	got := stripANSI(m.renderSinglePanePreview())
	if !strings.Contains(got, "Back to files") {
		t.Error("should contain 'Back to files' link")
	}
	if !strings.Contains(got, "README.md") {
		t.Error("should show the file name")
	}
	if !strings.Contains(got, "(no content)") {
		t.Error("should show '(no content)' for empty preview")
	}
}

func TestRenderSinglePanePreview_WithContent(t *testing.T) {
	m := splitViewModel{
		items: []splitViewItem{
			{Label: "main.go", Path: "/tmp/main.go"},
		},
		cursor:         0,
		showingPreview: true,
		previewContent: "package main\n\nfunc main() {\n}\n",
		width:          60,
		height:         20,
		zonePrefix:     "sv-test",
	}
	got := stripANSI(m.renderSinglePanePreview())
	if !strings.Contains(got, "main.go") {
		t.Error("should show file name")
	}
	if !strings.Contains(got, "package main") {
		t.Error("should show file content")
	}
	// Line numbers should appear
	if !strings.Contains(got, "1 ") {
		t.Error("should contain line number 1")
	}
}

func TestRenderSinglePanePreview_ScrollIndicators(t *testing.T) {
	// Create content that exceeds viewport
	lines := ""
	for i := 0; i < 50; i++ {
		lines += "line content\n"
	}
	m := splitViewModel{
		items: []splitViewItem{
			{Label: "big.txt", Path: "/tmp/big.txt"},
		},
		cursor:         0,
		showingPreview: true,
		previewContent: lines,
		previewScroll:  5,
		width:          60,
		height:         15,
		zonePrefix:     "sv-test",
	}
	got := stripANSI(m.renderSinglePanePreview())
	// With scroll offset > 0, should show scroll-up indicator
	if !strings.Contains(got, "above") {
		t.Error("should show scroll-up indicator when scrolled down")
	}
	// Content exceeds viewport, should show scroll-down indicator
	if !strings.Contains(got, "below") {
		t.Error("should show scroll-down indicator when more content below")
	}
}

func TestRenderSinglePanePreview_NilCursorItem(t *testing.T) {
	m := splitViewModel{
		items:          nil,
		cursor:         0,
		showingPreview: true,
		previewContent: "some text",
		width:          60,
		height:         20,
		zonePrefix:     "sv-test",
	}
	// Should not panic with nil cursor item
	got := stripANSI(m.renderSinglePanePreview())
	if !strings.Contains(got, "Back to files") {
		t.Error("should still show back link even with nil cursor item")
	}
}

// ---------------------------------------------------------------------------
// renderLoadoutContentsTab tests
// ---------------------------------------------------------------------------

func TestRenderLoadoutContentsTab_Error(t *testing.T) {
	m := detailModel{
		item:               catalog.ContentItem{Name: "test", Type: catalog.Loadouts},
		loadoutManifestErr: "parse error: invalid yaml",
		width:              60,
		height:             24,
	}
	got := stripANSI(m.renderLoadoutContentsTab())
	if !strings.Contains(got, "parse error") {
		t.Error("should show error message")
	}
}

func TestRenderLoadoutContentsTab_NilManifest(t *testing.T) {
	m := detailModel{
		item:            catalog.ContentItem{Name: "test", Type: catalog.Loadouts},
		loadoutManifest: nil,
		width:           60,
		height:          24,
	}
	got := stripANSI(m.renderLoadoutContentsTab())
	if !strings.Contains(got, "No loadout manifest") {
		t.Error("should show 'No loadout manifest' message")
	}
}

// ---------------------------------------------------------------------------
// renderLoadoutApplyTab tests
// ---------------------------------------------------------------------------

func TestRenderLoadoutApplyTab_Error(t *testing.T) {
	m := detailModel{
		item:               catalog.ContentItem{Name: "test", Type: catalog.Loadouts},
		loadoutManifestErr: "bad yaml",
		width:              60,
		height:             24,
	}
	got := stripANSI(m.renderLoadoutApplyTab())
	if !strings.Contains(got, "bad yaml") {
		t.Error("should show error message")
	}
}

func TestRenderLoadoutApplyTab_NilManifest(t *testing.T) {
	m := detailModel{
		item:            catalog.ContentItem{Name: "test", Type: catalog.Loadouts},
		loadoutManifest: nil,
		width:           60,
		height:          24,
	}
	got := stripANSI(m.renderLoadoutApplyTab())
	if !strings.Contains(got, "No loadout manifest") {
		t.Error("should show 'No loadout manifest' message")
	}
}

func TestRenderLoadoutApplyTab_WithManifest(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{Name: "test", Type: catalog.Loadouts},
		loadoutManifest: &loadout.Manifest{
			Kind:     "loadout",
			Version:  1,
			Provider: "claude-code",
			Name:     "test-loadout",
		},
		loadoutModeCursor: 0,
		width:             60,
		height:            24,
	}
	got := stripANSI(m.renderLoadoutApplyTab())
	if !strings.Contains(got, "Apply Loadout") {
		t.Error("should show 'Apply Loadout' heading")
	}
	if !strings.Contains(got, "Preview") {
		t.Error("should show Preview mode option")
	}
	if !strings.Contains(got, "Try") {
		t.Error("should show Try mode option")
	}
	if !strings.Contains(got, "Keep") {
		t.Error("should show Keep mode option")
	}
	if !strings.Contains(got, "Apply") {
		t.Error("should show Apply action button")
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
			tm := toastModel{active: true, text: tt.message, isErr: tt.isError, width: 60}
			view := tm.view()
			assertContains(t, view, tt.wantPrefix+" "+tt.message)
		})
	}
}
