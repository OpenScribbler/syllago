package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// TestAddWizard_DiscoveryMultiSelect exercises the monolithic-rule discovery
// step (D2, D18). Three mock candidates are seeded on the Discovery step with
// source set to addSourceMonolithic. Two spacebar presses at different cursor
// positions must leave both rows in selectedCandidates and render each with
// the selected checkbox mark. Tests the keyboard toggle behavior — the mouse
// click equivalent is covered in add_wizard_mouse_test.go (Task 4.8).
func TestAddWizard_DiscoveryMultiSelect(t *testing.T) {
	t.Parallel()

	m := openAddWizard(nil, nil, nil, "/tmp/proj", "/tmp/content", "")
	m.source = addSourceMonolithic
	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.step = addStepDiscovery
	m.width = 100
	m.height = 30
	m.discovering = false
	m.discoveryCandidates = []monolithicCandidate{
		{RelPath: "CLAUDE.md", Filename: "CLAUDE.md", Scope: "project", Lines: 40, H2Count: 5, ProviderID: "claude-code"},
		{RelPath: "AGENTS.md", Filename: "AGENTS.md", Scope: "project", Lines: 30, H2Count: 3, ProviderID: "codex"},
		{RelPath: "GEMINI.md", Filename: "GEMINI.md", Scope: "global", Lines: 50, H2Count: 6, ProviderID: "gemini-cli"},
	}
	m.discoveryCandidateCurs = 0

	// Press space at row 0
	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeySpace})

	// Move cursor to row 2 and press space
	m.discoveryCandidateCurs = 2
	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeySpace})

	if len(m.selectedCandidates) != 2 {
		t.Fatalf("expected 2 selected candidates, got %d (%v)", len(m.selectedCandidates), m.selectedCandidates)
	}
	gotSel := map[int]bool{}
	for _, idx := range m.selectedCandidates {
		gotSel[idx] = true
	}
	if !gotSel[0] {
		t.Errorf("expected row 0 to be selected")
	}
	if !gotSel[2] {
		t.Errorf("expected row 2 to be selected")
	}

	// Render the rows and verify the selected mark shows up for rows 0 and 2
	rows := m.renderMonolithicDiscoveryRows(m.width)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rendered rows, got %d", len(rows))
	}
	if !strings.Contains(rows[0], "[x]") {
		t.Errorf("row 0 should show [x] mark, got %q", rows[0])
	}
	if strings.Contains(rows[1], "[x]") {
		t.Errorf("row 1 should NOT show [x] mark, got %q", rows[1])
	}
	if !strings.Contains(rows[2], "[x]") {
		t.Errorf("row 2 should show [x] mark, got %q", rows[2])
	}
}

// TestAddWizard_DiscoveryInLibraryIndicator seeds a library with a single
// rule whose source.hash matches the canonical-hash of a discovery candidate.
// The rendered discovery row for that candidate must include the "✓ in library"
// indicator; other rows must not (D11, D18).
func TestAddWizard_DiscoveryInLibraryIndicator(t *testing.T) {
	t.Parallel()
	contentRoot := t.TempDir()

	// Seed a library rule whose source.hash corresponds to the bytes we'll
	// write to a CLAUDE.md file below.
	body := []byte("# Canonical rule\n\nThis file is already in the library.\n")
	srcBytes := []byte("# CLAUDE.md\n\nThis is the imported source file.\n")
	hash := rulestore.HashBody(srcBytes)
	meta := metadata.RuleMetadata{
		FormatVersion:  metadata.CurrentFormatVersion,
		Name:           "imported-rule",
		Type:           "rule",
		CurrentVersion: rulestore.HashBody(body),
		Versions:       []metadata.RuleVersionEntry{{Hash: rulestore.HashBody(body), WrittenAt: time.Now().UTC()}},
		Source: metadata.RuleSource{
			Provider: "claude-code",
			Scope:    "project",
			Path:     "/fake/CLAUDE.md",
			Format:   "claude-code",
			Filename: "CLAUDE.md",
			Hash:     hash,
		},
	}
	dir := filepath.Join(contentRoot, "claude-code", "imported-rule")
	if err := os.MkdirAll(filepath.Join(dir, ".history"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rule.md"), body, 0644); err != nil {
		t.Fatal(err)
	}
	hashFile := strings.Replace(rulestore.HashBody(body), ":", "-", 1) + ".md"
	if err := os.WriteFile(filepath.Join(dir, ".history", hashFile), body, 0644); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(&meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, metadata.FileName), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Build discovery candidates — mark one as in-library by computing
	// the same hash against its Bytes. discoverMonolithicCandidates would
	// do this; we seed directly for determinism.
	inLibHashes := loadLibraryRuleSourceHashes(contentRoot)
	if _, ok := inLibHashes[hash]; !ok {
		t.Fatalf("expected library hash %s loaded, got %v", hash, inLibHashes)
	}

	// Actually invoke discoverMonolithicCandidates against a real project
	// root so we exercise the hash-check logic end-to-end. Pin home dir to
	// an empty temp so user-level CLAUDE.md etc don't leak into the result.
	origHome := monolithicHomeDirOverride
	monolithicHomeDirOverride = t.TempDir()
	t.Cleanup(func() { monolithicHomeDirOverride = origHome })

	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "CLAUDE.md"), srcBytes, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "AGENTS.md"), []byte("# not in lib\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cands, derr := discoverMonolithicCandidates(projectRoot, contentRoot)
	if derr != nil {
		t.Fatalf("discover error: %v", derr)
	}
	var hitClaude bool
	var hitAgents bool
	for _, c := range cands {
		if c.Filename == "CLAUDE.md" {
			hitClaude = true
			if !c.InLibrary {
				t.Errorf("CLAUDE.md candidate should be marked InLibrary=true from hash match")
			}
		}
		if c.Filename == "AGENTS.md" {
			hitAgents = true
			if c.InLibrary {
				t.Errorf("AGENTS.md candidate should NOT be marked InLibrary")
			}
		}
	}
	if !hitClaude || !hitAgents {
		t.Fatalf("expected both CLAUDE.md and AGENTS.md in candidates, got %+v", cands)
	}

	m := openAddWizard(nil, nil, nil, projectRoot, contentRoot, "")
	m.source = addSourceMonolithic
	m.step = addStepDiscovery
	m.width = 100
	m.height = 30
	m.discovering = false
	m.discoveryCandidates = cands

	view := m.viewMonolithicDiscovery()
	// The row for CLAUDE.md must contain "✓ in library"
	lines := strings.Split(view, "\n")
	var claudeRow string
	var agentsRow string
	for _, ln := range lines {
		if strings.Contains(ln, "CLAUDE.md") {
			claudeRow = ln
		}
		if strings.Contains(ln, "AGENTS.md") {
			agentsRow = ln
		}
	}
	if !strings.Contains(claudeRow, "✓ in library") {
		t.Errorf("CLAUDE.md row should contain '✓ in library' indicator, got %q", claudeRow)
	}
	if strings.Contains(agentsRow, "✓ in library") {
		t.Errorf("AGENTS.md row should NOT contain '✓ in library' indicator, got %q", agentsRow)
	}

	// Sanity: keep tea.KeyMsg import even if we didn't use it
	_ = tea.KeyMsg{}
}
