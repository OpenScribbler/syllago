package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installcheck"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// testAppWithVerifiedRulesSize builds a TUI App with 2 library rules:
// one installed (MatchSet hit → ✓ in Installed column; PerRecord entry
// drives the Clean line in the metapanel breakdown), one never installed
// (blank Installed). Used by Phase 9 golden tests at 3 sizes.
func testAppWithVerifiedRulesSize(t *testing.T, w, h int) App {
	t.Helper()

	installedRule := catalog.ContentItem{
		Name: "installed-rule", Type: catalog.Rules, Source: "library",
		Library: true,
		Files:   []string{"rule.md"},
		Meta:    &metadata.Meta{ID: "lib-installed"},
	}
	freshRule := catalog.ContentItem{
		Name: "fresh-rule", Type: catalog.Rules, Source: "library",
		Library: true,
		Files:   []string{"rule.md"},
		Meta:    &metadata.Meta{ID: "lib-fresh"},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{installedRule, freshRule}}

	verification := &installcheck.VerificationResult{
		MatchSet: map[string][]string{
			"lib-installed": {"/tmp/project/CLAUDE.md"},
		},
		PerRecord: map[installcheck.RecordKey]installcheck.PerTargetState{
			{LibraryID: "lib-installed", TargetFile: "/tmp/project/CLAUDE.md"}: {
				State: installcheck.StateClean, Reason: installcheck.ReasonNone,
			},
		},
	}
	inst := &installer.Installed{
		RuleAppends: []installer.InstalledRuleAppend{
			{
				Name:        "installed-rule",
				LibraryID:   "lib-installed",
				Provider:    "claude-code",
				TargetFile:  "/tmp/project/CLAUDE.md",
				VersionHash: "sha256:abc",
				Source:      "manual",
			},
		},
	}

	app := NewApp(cat, testProviders(), "0.0.0-test", false, nil, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: w, Height: h})
	a := m.(App)
	a.verification = verification
	a.installed = inst
	a.refreshContent()
	return a
}

// TestGolden_LibraryInstalledRules verifies the library table renders the
// D16 binary Installed column (✓ for installed rules, blank for fresh
// rules). Cursor defaults to row 0 = fresh-rule (alphabetical), so the
// metapanel at the top of the screen shows a Not-Installed metapanel.
// Three sizes exercise responsive layout: 60x20 (minimum), 80x30 (default),
// 120x40 (wide).
func TestGolden_LibraryInstalledRules_60x20(t *testing.T) {
	app := testAppWithVerifiedRulesSize(t, 60, 20)
	requireGolden(t, "library-installed-rules-60x20", snapshotApp(t, app))
}

func TestGolden_LibraryInstalledRules_80x30(t *testing.T) {
	app := testAppWithVerifiedRulesSize(t, 80, 30)
	requireGolden(t, "library-installed-rules-80x30", snapshotApp(t, app))
}

func TestGolden_LibraryInstalledRules_120x40(t *testing.T) {
	app := testAppWithVerifiedRulesSize(t, 120, 40)
	requireGolden(t, "library-installed-rules-120x40", snapshotApp(t, app))
}

// TestGolden_MetapanelRuleInstalled verifies the metapanel shows the D16
// per-target breakdown when an installed rule is selected. Pressing
// `down` moves the cursor to row 1 = installed-rule, which triggers the
// "Installed at:" section with one Clean record.
func TestGolden_MetapanelRuleInstalled_60x20(t *testing.T) {
	app := testAppWithVerifiedRulesSize(t, 60, 20)
	m, _ := app.Update(keyPress(tea.KeyDown))
	app = m.(App)
	requireGolden(t, "metapanel-rule-installed-60x20", snapshotApp(t, app))
}

func TestGolden_MetapanelRuleInstalled_80x30(t *testing.T) {
	app := testAppWithVerifiedRulesSize(t, 80, 30)
	m, _ := app.Update(keyPress(tea.KeyDown))
	app = m.(App)
	requireGolden(t, "metapanel-rule-installed-80x30", snapshotApp(t, app))
}

func TestGolden_MetapanelRuleInstalled_120x40(t *testing.T) {
	app := testAppWithVerifiedRulesSize(t, 120, 40)
	m, _ := app.Update(keyPress(tea.KeyDown))
	app = m.(App)
	requireGolden(t, "metapanel-rule-installed-120x40", snapshotApp(t, app))
}
