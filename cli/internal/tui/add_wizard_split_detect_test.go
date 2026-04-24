package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// writeSplittableClaudeMD creates a CLAUDE.md in dir that passes the H2
// splitter pre-check (≥30 lines + ≥3 H2 headings). Returns the full path.
func writeSplittableClaudeMD(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("# Project\n")
	for i := 0; i < 3; i++ {
		b.WriteString("## Section ")
		b.WriteByte(byte('A' + i))
		b.WriteString("\n")
		for j := 0; j < 12; j++ {
			b.WriteString("body line of content here\n")
		}
	}
	path := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestAddWizard_DiscoveryFlagsSplittableRules verifies that handleDiscoveryDone
// annotates rule items with splittable=true when their source file matches a
// monolithic filename and passes the H2 splitter pre-check.
func TestAddWizard_DiscoveryFlagsSplittableRules(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	claudePath := writeSplittableClaudeMD(t, tmp)

	m := testOpenAddWizard(t)
	items := []addDiscoveryItem{
		{
			name: "CLAUDE", itemType: catalog.Rules,
			status:     add.StatusNew,
			path:       claudePath,
			underlying: &add.DiscoveryItem{Name: "CLAUDE", Type: catalog.Rules, Path: claudePath, Status: add.StatusNew},
		},
		{
			name: "regular-rule", itemType: catalog.Rules,
			status:     add.StatusNew,
			path:       filepath.Join(tmp, "does-not-exist.md"),
			underlying: &add.DiscoveryItem{Name: "regular-rule", Type: catalog.Rules, Path: filepath.Join(tmp, "does-not-exist.md"), Status: add.StatusNew},
		},
	}
	m = injectDiscoveryResults(t, m, items)

	// New (StatusNew) items flow into confirmItems for triage before merging
	// into discoveredItems. Check annotations at that intermediate stage so the
	// test verifies the handler, not the full wizard advance path.
	var claudeItem *addConfirmItem
	var regularItem *addConfirmItem
	for i := range m.confirmItems {
		switch m.confirmItems[i].displayName {
		case "CLAUDE":
			claudeItem = &m.confirmItems[i]
		case "regular-rule":
			regularItem = &m.confirmItems[i]
		}
	}
	if claudeItem == nil {
		t.Fatal("CLAUDE item missing after discovery")
	}
	if !claudeItem.splittable {
		t.Fatal("CLAUDE.md must be flagged splittable")
	}
	if claudeItem.splitSectionCount != 3 {
		t.Fatalf("expected 3 sections, got %d", claudeItem.splitSectionCount)
	}
	if regularItem == nil {
		t.Fatal("regular-rule item missing after discovery")
	}
	if regularItem.splittable {
		t.Fatal("non-monolithic rule must not be splittable")
	}
}

// TestAddWizard_ReviewShowsSplittableHint verifies that when a splittable rule
// is selected in Review, the view renders a hint line naming the section count.
func TestAddWizard_ReviewShowsSplittableHint(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	claudePath := writeSplittableClaudeMD(t, tmp)

	m := testOpenAddWizard(t)
	items := []addDiscoveryItem{{
		name: "CLAUDE", itemType: catalog.Rules,
		status:     add.StatusNew,
		path:       claudePath,
		underlying: &add.DiscoveryItem{Name: "CLAUDE", Type: catalog.Rules, Path: claudePath, Status: add.StatusNew},
	}}
	m = injectDiscoveryResults(t, m, items)

	// Simulate user confirming triage: select the auto-confirmed item and
	// merge it into discoveredItems as enterReview() would.
	for i := range m.confirmSelected {
		m.confirmSelected[i] = true
	}
	m.mergeConfirmIntoDiscovery()

	m.step = addStepReview
	m.shell.SetActive(m.shellIndexForStep(addStepReview))
	for i := range m.discoveryList.selected {
		m.discoveryList.selected[i] = true
	}

	out := m.View()
	if !strings.Contains(out, "Detected 1 monolithic rule") {
		t.Fatalf("expected splittable hint in Review; got:\n%s", out)
	}
	if !strings.Contains(out, "3 sections") {
		t.Fatalf("expected section count in hint; got:\n%s", out)
	}
}
