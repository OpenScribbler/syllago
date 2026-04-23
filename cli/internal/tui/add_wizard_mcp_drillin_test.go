package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestMCPDrillIn_ExtractsSnippetNotFullFile verifies that drilling into an MCP
// item in the review step shows only the relevant server snippet, not the full
// settings file.
func TestMCPDrillIn_ExtractsSnippetNotFullFile(t *testing.T) {
	t.Parallel()

	// Build a fake settings file with multiple MCP servers.
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")
	settings := map[string]any{
		"mcpServers": map[string]any{
			"puppeteer": map[string]any{
				"command": "npx",
				"args":    []string{"-y", "@modelcontextprotocol/server-puppeteer"},
			},
			"other": map[string]any{
				"command": "node",
				"args":    []string{"/some/path"},
			},
		},
		"someOtherKey": "lots of other data that should NOT appear in the drill-in",
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Construct the addDiscoveryItem as mergeConfirmIntoDiscovery produces it:
	// path = full settings file path, name = server key.
	di := addDiscoveryItem{
		name:        "puppeteer",
		displayName: "puppeteer",
		itemType:    catalog.MCP,
		path:        settingsPath,
		sourceDir:   tmp,
		status:      add.StatusNew,
		underlying: &add.DiscoveryItem{
			Name: "puppeteer",
			Type: catalog.MCP,
			Path: settingsPath,
		},
	}
	// Synthesize catalogItem as mergeConfirmIntoDiscovery does (else-if branch).
	ci := catalog.ContentItem{
		Name:  "puppeteer",
		Type:  catalog.MCP,
		Path:  tmp,
		Files: []string{"settings.json"},
	}
	di.catalogItem = &ci

	m := testOpenAddWizard(t)
	m.step = addStepReview
	m.width = 120
	m.height = 40
	m.discoveredItems = []addDiscoveryItem{di}
	m.actionableCount = 1
	m.installedCount = 0
	m.discoveryList = m.buildDiscoveryList()
	m.reviewItemCursor = 0
	m.reviewZone = addReviewZoneItems

	m.enterReviewDrillIn()

	if !m.reviewDrillIn {
		t.Fatal("drill-in did not open")
	}
	if len(m.reviewDrillPreview.lines) == 0 {
		t.Fatal("preview is empty")
	}

	preview := strings.Join(m.reviewDrillPreview.lines, "\n")

	if !strings.Contains(preview, "puppeteer") {
		t.Errorf("preview missing puppeteer key:\n%s", preview)
	}
	if strings.Contains(preview, "someOtherKey") {
		t.Errorf("preview contains unrelated content from settings file:\n%s", preview)
	}

	if len(m.reviewDrillTree.nodes) != 1 {
		t.Fatalf("expected 1 tree node, got %d", len(m.reviewDrillTree.nodes))
	}
	if m.reviewDrillTree.nodes[0].path != "puppeteer.json" {
		t.Errorf("tree node path = %q, want %q", m.reviewDrillTree.nodes[0].path, "puppeteer.json")
	}
}
