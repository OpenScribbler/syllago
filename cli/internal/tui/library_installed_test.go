package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installcheck"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// TestLibrary_InstalledColumnBinary verifies the D16 "Installed" column is
// binary (Installed / Not-Installed) for rules. When MatchSet has a non-empty
// entry for an item's library ID, the column renders a ✓ glyph in successColor
// styling; otherwise it renders a blank / "--".
func TestLibrary_InstalledColumnBinary(t *testing.T) {
	t.Parallel()

	installedRule := catalog.ContentItem{
		Name:    "installed-rule",
		Type:    catalog.Rules,
		Source:  "library",
		Library: true,
		Files:   []string{"rule.md"},
		Meta:    &metadata.Meta{ID: "lib-installed"},
	}
	notInstalledRule := catalog.ContentItem{
		Name:    "fresh-rule",
		Type:    catalog.Rules,
		Source:  "library",
		Library: true,
		Files:   []string{"rule.md"},
		Meta:    &metadata.Meta{ID: "lib-fresh"},
	}

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

	l := newLibraryModel([]catalog.ContentItem{installedRule, notInstalledRule}, nil, "")
	l.SetVerification(verification)
	l.SetSize(140, 30)

	// Force deterministic ordering: sort by name so row 0 = fresh-rule,
	// row 1 = installed-rule (alphabetical). We use that ordering in
	// assertions below.
	rows := make([]string, 0, l.table.Len())
	view := ansi.Strip(l.viewBrowse())
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "installed-rule") || strings.Contains(line, "fresh-rule") {
			rows = append(rows, line)
		}
	}
	if len(rows) < 2 {
		t.Fatalf("expected 2 rule rows in the rendered library, got %d:\n%s", len(rows), view)
	}

	var installedLine, freshLine string
	for _, r := range rows {
		switch {
		case strings.Contains(r, "installed-rule"):
			installedLine = r
		case strings.Contains(r, "fresh-rule"):
			freshLine = r
		}
	}
	if installedLine == "" {
		t.Fatalf("missing installed-rule row; rendered:\n%s", view)
	}
	if freshLine == "" {
		t.Fatalf("missing fresh-rule row; rendered:\n%s", view)
	}

	// Installed rule shows ✓ in its Installed column.
	if !strings.Contains(installedLine, "✓") {
		t.Errorf("installed-rule row should contain ✓ in Installed column, got:\n%s", installedLine)
	}
	// Fresh rule does not show ✓ — it shows "--" or blank.
	if strings.Contains(freshLine, "✓") {
		t.Errorf("fresh-rule row should NOT contain ✓ in Installed column, got:\n%s", freshLine)
	}
}
