package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestAddWizard_Init_ReturnsNil(t *testing.T) {
	t.Parallel()
	m := &addWizardModel{}
	if cmd := m.Init(); cmd != nil {
		t.Errorf("expected nil cmd from Init, got %v", cmd)
	}
}

func TestAddWizard_ReviewTabForwardBackward(t *testing.T) {
	t.Parallel()
	m := &addWizardModel{reviewZone: addReviewZoneItems}

	// Forward: items -> buttons
	m.reviewTabForward()
	if m.reviewZone != addReviewZoneButtons {
		t.Errorf("forward from items: got %v, want buttons", m.reviewZone)
	}
	// Forward: buttons -> items (wrap)
	m.reviewTabForward()
	if m.reviewZone != addReviewZoneItems {
		t.Errorf("forward from buttons: got %v, want items (wrap)", m.reviewZone)
	}
	// Backward: items -> buttons (wrap)
	m.reviewTabBackward()
	if m.reviewZone != addReviewZoneButtons {
		t.Errorf("backward from items: got %v, want buttons (wrap)", m.reviewZone)
	}
	// Backward: buttons -> items
	m.reviewTabBackward()
	if m.reviewZone != addReviewZoneItems {
		t.Errorf("backward from buttons: got %v, want items", m.reviewZone)
	}
}

func TestAddWizard_ReviewTabForward_UnknownZoneDefaultsToFirst(t *testing.T) {
	t.Parallel()
	// Use a zone value that's not in reviewZoneOrder() (the legacy Risks zone).
	m := &addWizardModel{reviewZone: addReviewZoneRisks}
	m.reviewTabForward()
	if m.reviewZone != addReviewZoneItems {
		t.Errorf("forward from unknown zone: got %v, want items (first in order)", m.reviewZone)
	}
}

func TestAddWizard_ReviewTabBackward_UnknownZoneDefaultsToLast(t *testing.T) {
	t.Parallel()
	m := &addWizardModel{reviewZone: addReviewZoneRisks}
	m.reviewTabBackward()
	if m.reviewZone != addReviewZoneButtons {
		t.Errorf("backward from unknown zone: got %v, want buttons (last in order)", m.reviewZone)
	}
}

func TestExtractJSONSection_MissingFile(t *testing.T) {
	t.Parallel()
	got := extractJSONSection("/nonexistent/path/settings.json", "foo", catalog.MCP)
	if got != "" {
		t.Errorf("expected empty for missing file, got %q", got)
	}
}

func TestExtractJSONSection_MCP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	raw := map[string]any{
		"mcpServers": map[string]any{
			"my-server": map[string]any{
				"command": "my-cmd",
				"args":    []string{"--help"},
			},
		},
	}
	data, _ := json.Marshal(raw)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	got := extractJSONSection(path, "my-server", catalog.MCP)
	if got == "" {
		t.Fatal("expected non-empty result for existing MCP server")
	}
	if !stringHas(got, "my-server") {
		t.Errorf("expected result to include server name, got %q", got)
	}
	if !stringHas(got, "my-cmd") {
		t.Errorf("expected result to include command, got %q", got)
	}

	// Missing server name returns empty.
	if out := extractJSONSection(path, "nope", catalog.MCP); out != "" {
		t.Errorf("expected empty for missing server, got %q", out)
	}
}

func TestExtractJSONSection_MCP_NestedMcpField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	raw := map[string]any{
		"mcp": map[string]any{
			"mcpServers": map[string]any{
				"nested": map[string]any{"command": "cmd"},
			},
		},
	}
	data, _ := json.Marshal(raw)
	_ = os.WriteFile(path, data, 0644)
	got := extractJSONSection(path, "nested", catalog.MCP)
	if got == "" {
		t.Fatal("expected non-empty result via nested path mcp.mcpServers")
	}
}

func TestExtractJSONSection_Hooks_EmptyWhenNoHooksKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"other": 1}`), 0644); err != nil {
		t.Fatal(err)
	}
	got := extractJSONSection(path, "any", catalog.Hooks)
	if got != "" {
		t.Errorf("expected empty for file without hooks key, got %q", got)
	}
}

func TestExtractJSONSection_UnknownContentType_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	_ = os.WriteFile(path, []byte(`{"ok": true}`), 0644)
	got := extractJSONSection(path, "foo", catalog.Rules)
	if got != "" {
		t.Errorf("expected empty for unhandled content type, got %q", got)
	}
}

func TestScanDrillInFiles_NonExistent(t *testing.T) {
	t.Parallel()
	got := scanDrillInFiles("/nonexistent/path")
	if got != nil {
		t.Errorf("expected nil for nonexistent path, got %v", got)
	}
}

func TestScanDrillInFiles_SingleFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "solo.md")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	got := scanDrillInFiles(path)
	if len(got) != 1 || got[0] != "solo.md" {
		t.Errorf("expected [solo.md], got %v", got)
	}
}

func TestScanDrillInFiles_Directory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Visible files
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	// Hidden file + hidden dir (both should be skipped)
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h"), 0644); err != nil {
		t.Fatal(err)
	}
	hiddenDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "deep"), []byte("g"), 0644); err != nil {
		t.Fatal(err)
	}

	got := scanDrillInFiles(dir)
	hasA, hasB := false, false
	for _, f := range got {
		if f == "a.md" {
			hasA = true
		}
		if f == filepath.Join("sub", "b.txt") {
			hasB = true
		}
		if f == ".hidden" || f == filepath.Join(".git", "deep") {
			t.Errorf("expected hidden file/dir to be skipped, got %q", f)
		}
	}
	if !hasA || !hasB {
		t.Errorf("expected a.md and sub/b.txt in result, got %v", got)
	}
}

func TestHighlightRisksForItem_FiltersByItemIdx(t *testing.T) {
	t.Parallel()
	m := &addWizardModel{
		riskItemMap: []int{0, 1, 0, 2, 1},
		riskBanner: newRiskBanner([]catalog.RiskIndicator{
			{Label: "r0", Level: catalog.RiskMedium},
			{Label: "r1", Level: catalog.RiskMedium},
			{Label: "r2", Level: catalog.RiskMedium},
			{Label: "r3", Level: catalog.RiskMedium},
			{Label: "r4", Level: catalog.RiskMedium},
		}, 60),
	}

	m.highlightRisksForItem(1)
	// Indices 1 and 4 map to item 1
	if !m.riskBanner.highlighted[1] || !m.riskBanner.highlighted[4] {
		t.Errorf("expected indices 1 and 4 highlighted for item 1, got %v", m.riskBanner.highlighted)
	}
	if m.riskBanner.highlighted[0] || m.riskBanner.highlighted[2] {
		t.Errorf("expected indices 0 and 2 not highlighted for item 1, got %v", m.riskBanner.highlighted)
	}

	// No matches → highlight cleared
	m.highlightRisksForItem(99)
	if m.riskBanner.highlighted != nil {
		t.Errorf("expected highlighted cleared when no matches, got %v", m.riskBanner.highlighted)
	}
}

func TestTriageItemVisualRow(t *testing.T) {
	t.Parallel()
	items := []addConfirmItem{
		{itemType: catalog.Skills},
		{itemType: catalog.Skills},
		{itemType: catalog.Agents}, // new group header +1
		{itemType: catalog.Rules},  // new group header +1
	}
	cases := []struct {
		idx  int
		want int
	}{
		{0, 1}, // header + item 0 → row 1
		{1, 2}, // same group → row 2
		{2, 4}, // new group header pushes item 2 to row 4
		{3, 6}, // new group header pushes item 3 to row 6
	}
	for _, c := range cases {
		if got := triageItemVisualRow(items, c.idx); got != c.want {
			t.Errorf("triageItemVisualRow(%d) = %d, want %d", c.idx, got, c.want)
		}
	}

	// Empty → 0
	if got := triageItemVisualRow(nil, 0); got != 0 {
		t.Errorf("empty slice: got %d, want 0", got)
	}
}

func TestAdjustTriageOffset(t *testing.T) {
	t.Parallel()
	m := &addWizardModel{
		height: 30, // vh = max(3, 30-12) = 18
		confirmItems: []addConfirmItem{
			{itemType: catalog.Skills},
			{itemType: catalog.Skills},
			{itemType: catalog.Agents},
			{itemType: catalog.Rules},
		},
		confirmCursor: 0,
		confirmOffset: 20, // far below visible row 1
	}
	m.adjustTriageOffset()
	// Cursor row 1 is < offset 20 → offset should snap down to 1
	if m.confirmOffset != 1 {
		t.Errorf("expected offset=1 after cursor above, got %d", m.confirmOffset)
	}

	// Cursor far below offset window
	m.confirmCursor = 3 // visual row 6
	m.confirmOffset = 0
	m.height = 8 // vh = max(3, 8-12) = 3
	m.adjustTriageOffset()
	// row 6 >= offset(0) + 3 → offset = 6 - 3 + 1 = 4
	if m.confirmOffset != 4 {
		t.Errorf("expected offset=4 after cursor below window, got %d", m.confirmOffset)
	}
}

func TestSortConfirmItemsByType(t *testing.T) {
	t.Parallel()
	items := []addConfirmItem{
		{displayName: "rule1", itemType: catalog.Rules},
		{displayName: "skill1", itemType: catalog.Skills},
		{displayName: "rule2", itemType: catalog.Rules},
		{displayName: "skill2", itemType: catalog.Skills},
	}
	selected := map[int]bool{1: true, 2: true} // skill1, rule2

	sorted, newSel := sortConfirmItemsByType(items, selected)
	// Skills (0) should come before Rules (2)
	if sorted[0].itemType != catalog.Skills || sorted[1].itemType != catalog.Skills {
		t.Errorf("expected Skills first in sorted, got %v, %v", sorted[0].itemType, sorted[1].itemType)
	}
	if sorted[2].itemType != catalog.Rules || sorted[3].itemType != catalog.Rules {
		t.Errorf("expected Rules next in sorted, got %v, %v", sorted[2].itemType, sorted[3].itemType)
	}
	// Stable sort: skill1 then skill2, rule1 then rule2
	if sorted[0].displayName != "skill1" || sorted[1].displayName != "skill2" {
		t.Errorf("skills order: got %q, %q; want skill1, skill2", sorted[0].displayName, sorted[1].displayName)
	}
	// Selection remapping: skill1 was at orig 1 → new 0; rule2 was at orig 2 → new 3
	if !newSel[0] || !newSel[3] {
		t.Errorf("selection remapping wrong: got %v", newSel)
	}
	if newSel[1] || newSel[2] {
		t.Errorf("unexpected indices selected: got %v", newSel)
	}
}

func TestAddWizard_StepHints_AllBranches(t *testing.T) {
	t.Parallel()
	containsHint := func(hints []string, substr string) bool {
		for _, h := range hints {
			for i := 0; i+len(substr) <= len(h); i++ {
				if h[i:i+len(substr)] == substr {
					return true
				}
			}
		}
		return false
	}

	cases := []struct {
		name  string
		setup func(m *addWizardModel)
		want  string // substring that should appear
	}{
		{"source default", func(m *addWizardModel) { m.step = addStepSource }, "close wizard"},
		{"source expanded", func(m *addWizardModel) { m.step = addStepSource; m.sourceExpanded = true }, "collapse"},
		{"source input", func(m *addWizardModel) { m.step = addStepSource; m.inputActive = true }, "path/URL"},
		{"type", func(m *addWizardModel) { m.step = addStepType }, "space toggle"},
		{"discovery scanning", func(m *addWizardModel) { m.step = addStepDiscovery; m.discovering = true }, "cancel"},
		{"discovery error", func(m *addWizardModel) { m.step = addStepDiscovery; m.discoveryErr = "boom" }, "retry"},
		{"discovery empty", func(m *addWizardModel) { m.step = addStepDiscovery }, "back"},
		{"discovery full", func(m *addWizardModel) {
			m.step = addStepDiscovery
			m.confirmItems = []addConfirmItem{{itemType: catalog.Skills}}
		}, "switch panes"},
		{"review default", func(m *addWizardModel) { m.step = addStepReview }, "cycle zones"},
		{"review drill-in", func(m *addWizardModel) { m.step = addStepReview; m.reviewDrillIn = true }, "switch panes"},
		{"execute in progress", func(m *addWizardModel) { m.step = addStepExecute }, "cancel"},
		{"execute done", func(m *addWizardModel) { m.step = addStepExecute; m.executeDone = true }, "add more"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := &addWizardModel{}
			c.setup(m)
			got := m.stepHints()
			if len(got) == 0 {
				t.Fatalf("%s: got empty hints", c.name)
			}
			if !containsHint(got, c.want) {
				t.Errorf("%s: expected hints to contain %q, got %v", c.name, c.want, got)
			}
		})
	}
}

func stringHas(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
