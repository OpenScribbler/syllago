package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
)

// TestTriagePreview_HookConfirmItem verifies that loadTriagePreview renders
// hook.json content via hookData when the confirm item is a hook.
func TestTriagePreview_HookConfirmItem(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	hookItem := addDiscoveryItem{
		name:     "lint-hook",
		itemType: catalog.Hooks,
		path:     "/home/user/.claude/settings.json",
		status:   add.StatusNew,
		hookData: &converter.HookData{
			Event: "before_tool_execute",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "echo lint"},
			},
		},
		hookSourceDir: "/home/user/.claude",
		underlying: &add.DiscoveryItem{
			Name:   "lint-hook",
			Type:   catalog.Hooks,
			Path:   "/home/user/.claude/settings.json",
			Status: add.StatusNew,
		},
	}

	m = injectDiscoveryResults(t, m, []addDiscoveryItem{hookItem})
	if len(m.confirmItems) == 0 {
		t.Fatal("expected hook item in confirmItems")
	}

	// Confirm item must carry hookData through handleDiscoveryDone conversion.
	ci := m.confirmItems[0]
	if ci.hookData == nil {
		t.Fatal("confirmItem.hookData must be populated after handleDiscoveryDone")
	}

	m.confirmCursor = 0
	m.loadTriagePreview()

	if len(m.confirmPreview.lines) == 0 {
		t.Fatal("expected non-empty preview for hook confirm item")
	}
	joined := strings.Join(m.confirmPreview.lines, "\n")
	if !strings.Contains(joined, "before_tool_execute") && !strings.Contains(joined, "PreToolUse") {
		t.Errorf("hook preview missing event name; got:\n%s", joined)
	}
	if m.confirmPreview.fileName != "hook.json" {
		t.Errorf("expected fileName = hook.json, got %q", m.confirmPreview.fileName)
	}
}

// TestTriagePreview_FileItemDerivedSourceDir verifies that items whose
// addDiscoveryItem.sourceDir is empty get a correct sourceDir derived from
// filepath.Dir(path) so the triage preview can read the file.
func TestTriagePreview_FileItemDerivedSourceDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "# My Rule\nThis is a rule."
	fname := "my-rule.md"
	if err := os.WriteFile(filepath.Join(dir, fname), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m := testOpenAddWizard(t)

	// sourceDir intentionally left empty to simulate file items from
	// nativeItemsToDiscovery / DiscoverFromProvider for single-file items.
	ruleItem := addDiscoveryItem{
		name:      "my-rule",
		itemType:  catalog.Rules,
		path:      filepath.Join(dir, fname), // full absolute path, no sourceDir
		sourceDir: "",
		status:    add.StatusNew,
		underlying: &add.DiscoveryItem{
			Name:   "my-rule",
			Type:   catalog.Rules,
			Path:   filepath.Join(dir, fname),
			Status: add.StatusNew,
		},
	}

	m = injectDiscoveryResults(t, m, []addDiscoveryItem{ruleItem})
	if len(m.confirmItems) == 0 {
		t.Fatal("expected item in confirmItems")
	}

	ci := m.confirmItems[0]
	if ci.sourceDir == "" {
		t.Fatal("confirmItem.sourceDir must be derived from filepath.Dir(path) when empty")
	}
	if ci.sourceDir != dir {
		t.Errorf("expected sourceDir %q, got %q", dir, ci.sourceDir)
	}
	if ci.path != fname {
		t.Errorf("expected path %q, got %q", fname, ci.path)
	}

	m.confirmCursor = 0
	m.loadTriagePreview()

	if len(m.confirmPreview.lines) == 0 {
		t.Fatal("expected non-empty preview; sourceDir derivation must enable ReadFileContent")
	}
	if !strings.Contains(strings.Join(m.confirmPreview.lines, "\n"), "My Rule") {
		t.Error("preview content does not contain expected text")
	}
}

// TestTriagePreview_TierDefaultsToHighForPatternItems verifies that items
// discovered via known layout (tier == "") are assigned TierHigh.
func TestTriagePreview_TierDefaultsToHighForPatternItems(t *testing.T) {
	t.Parallel()

	m := testOpenAddWizard(t)
	ruleItem := addDiscoveryItem{
		name:            "pattern-rule",
		itemType:        catalog.Rules,
		path:            "/tmp/rules/pattern-rule.md",
		status:          add.StatusNew,
		tier:            "", // no tier = pattern-detected
		detectionSource: "", // not "content-signal"
		underlying: &add.DiscoveryItem{
			Name:   "pattern-rule",
			Type:   catalog.Rules,
			Path:   "/tmp/rules/pattern-rule.md",
			Status: add.StatusNew,
		},
	}
	m = injectDiscoveryResults(t, m, []addDiscoveryItem{ruleItem})
	if len(m.confirmItems) == 0 {
		t.Fatal("expected item in confirmItems")
	}
	if m.confirmItems[0].tier != analyzer.TierHigh {
		t.Errorf("expected TierHigh for pattern item, got %q", m.confirmItems[0].tier)
	}
}

// TestTriagePreview_AnalyzerTierPreserved verifies that items from the
// content-signal analyzer keep their original tier (not overridden to TierHigh).
func TestTriagePreview_AnalyzerTierPreserved(t *testing.T) {
	t.Parallel()

	m := testOpenAddWizard(t)
	ruleItem := addDiscoveryItem{
		name:            "analyzer-rule",
		itemType:        catalog.Rules,
		path:            "/tmp/rules/analyzer-rule.md",
		status:          add.StatusNew,
		tier:            analyzer.TierMedium,
		detectionSource: "content-signal",
		underlying: &add.DiscoveryItem{
			Name:   "analyzer-rule",
			Type:   catalog.Rules,
			Path:   "/tmp/rules/analyzer-rule.md",
			Status: add.StatusNew,
		},
	}
	m = injectDiscoveryResults(t, m, []addDiscoveryItem{ruleItem})
	if len(m.confirmItems) == 0 {
		t.Fatal("expected item in confirmItems")
	}
	if m.confirmItems[0].tier != analyzer.TierMedium {
		t.Errorf("expected TierMedium preserved for analyzer item, got %q", m.confirmItems[0].tier)
	}
}

// TestMergeConfirmIntoDiscovery_PreservesHookData verifies that hookData
// survives the addConfirmItem → addDiscoveryItem merge so Review drill-in works.
func TestMergeConfirmIntoDiscovery_PreservesHookData(t *testing.T) {
	t.Parallel()

	m := testOpenAddWizard(t)
	hookItem := addDiscoveryItem{
		name:     "preserve-hook",
		itemType: catalog.Hooks,
		path:     "/home/user/.claude/settings.json",
		status:   add.StatusNew,
		hookData: &converter.HookData{
			Event: "post_tool_execute",
			Hooks: []converter.HookEntry{{Type: "command", Command: "echo done"}},
		},
		hookSourceDir: "/home/user/.claude",
		underlying: &add.DiscoveryItem{
			Name:   "preserve-hook",
			Type:   catalog.Hooks,
			Path:   "/home/user/.claude/settings.json",
			Status: add.StatusNew,
		},
	}

	m = injectDiscoveryResults(t, m, []addDiscoveryItem{hookItem})
	m.mergeConfirmIntoDiscovery()

	// Find the hook in the merged discoveredItems
	var found *addDiscoveryItem
	for i := range m.discoveredItems {
		if m.discoveredItems[i].itemType == catalog.Hooks {
			found = &m.discoveredItems[i]
			break
		}
	}
	if found == nil {
		t.Fatal("hook item not found in discoveredItems after merge")
	}
	if found.hookData == nil {
		t.Fatal("hookData must survive mergeConfirmIntoDiscovery so Review drill-in works")
	}
	if found.hookData.Event != "post_tool_execute" {
		t.Errorf("hookData.Event corrupted: got %q", found.hookData.Event)
	}
}
