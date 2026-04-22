package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Responsive-layout invariants complementing the size-variant goldens.
//
// Goldens freeze-dry the rendered output and fail on any diff, but without
// an independent signal they can't distinguish a legitimate visual change
// from a layout regression. These tests pin the *structural* invariants
// behind the goldens: terminal-width clipping, narrow/wide column-count
// breakpoints, and truncation behavior in fixed-width columns. A regression
// like accidentally swapping MaxWidth() for Width() (per project memory:
// "lipgloss Width() word-wraps; MaxWidth() truncates") shows up here as a
// targeted failure rather than requiring a human to eyeball a golden diff.

// TestResponsive_FramedLinesNeverExceedTerminal verifies that every rendered
// line inside the main frame (topbar + content area; identified by a vertical
// border rune "│") fits within the terminal width at each supported size.
// lipgloss.Width computes visible width (rune-aware, counts wide chars
// correctly) on the ANSI-stripped snapshot.
//
// A MaxWidth→Width regression in table/metapanel/modal rendering would
// surface here as a line-width overrun — e.g., a header that normally
// truncates to 78 runes word-wrapping to a second line whose own width
// breaks the frame.
//
// The helpbar footer lines are intentionally excluded: at narrow widths its
// overflow row renders unwrapped to the full hint text, which is a known
// behavior of helpBarModel.View() (helpbar.go) and not part of the frame
// layout this test is guarding.
func TestResponsive_FramedLinesNeverExceedTerminal(t *testing.T) {
	cases := []struct {
		name     string
		w, h     int
		buildApp func(t *testing.T, w, h int) App
	}{
		{"shell_empty_80x30", 80, 30, testAppSize},
		{"shell_empty_120x40", 120, 40, testAppSize},
		{"library_items_80x30", 80, 30, testAppWithItemsSize},
		{"library_items_120x40", 120, 40, testAppWithItemsSize},
		{"moat_library_80x30", 80, 30, testAppWithMOATItems},
		{"moat_library_120x40", 120, 40, testAppWithMOATItems},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := tc.buildApp(t, tc.w, tc.h)
			snap := snapshotApp(t, app)
			inspected := 0
			for i, line := range strings.Split(snap, "\n") {
				if !strings.ContainsAny(line, "│╭╮╰╯├┤┬┴") {
					continue
				}
				inspected++
				if got := lipgloss.Width(line); got > tc.w {
					t.Errorf("framed line %d exceeds terminal width (want<=%d got=%d): %q",
						i+1, tc.w, got, line)
				}
			}
			if inspected == 0 {
				t.Fatalf("no framed lines found — test would be vacuous; snapshot:\n%s", snap)
			}
		})
	}
}

// TestResponsive_LibraryColumnCount_ByWidth pins the Description-column
// breakpoint in tableModel.columnWidths() (table.go:479): the Description
// column appears iff the table's usable width (terminal width - frame) is
// at least 100, which holds at 120-wide terminals but not at 80. A
// regression that moved the threshold or dropped the narrow fallback would
// ship a broken responsive layout the goldens catch only via a human diff.
func TestResponsive_LibraryColumnCount_ByWidth(t *testing.T) {
	narrow := snapshotApp(t, testAppWithItemsSize(t, 80, 30))
	wide := snapshotApp(t, testAppWithItemsSize(t, 120, 40))

	narrowCols := []string{"Name", "Type", "Scope", "Files", "Installed"}
	for _, col := range narrowCols {
		if !strings.Contains(narrow, col) {
			t.Errorf("80x30 library missing expected header %q:\n%s", col, narrow)
		}
	}
	if strings.Contains(narrow, "Description") {
		t.Errorf("80x30 library unexpectedly shows Description column (narrow mode should drop it):\n%s", narrow)
	}

	wideCols := append(narrowCols, "Description")
	for _, col := range wideCols {
		if !strings.Contains(wide, col) {
			t.Errorf("120x40 library missing expected header %q:\n%s", col, wide)
		}
	}
}

// TestResponsive_MOATLibraryScope_TruncatesWithAsciiDots verifies the
// fixed-width (12-rune) Scope column (table.go:475) truncates long source
// names at both narrow and wide terminal sizes. The truncate() helper in
// items.go:289 uses three ASCII dots "..." — not U+2026 "…" — so tests must
// match the rendered character or they'll pass today by coincidence and
// silently allow ellipsis-style regressions.
//
// The MOAT fixture has three items with Registry="moat-registry" (13 runes,
// one over the column width), so truncation must fire exactly once per row.
func TestResponsive_MOATLibraryScope_TruncatesWithAsciiDots(t *testing.T) {
	const truncated = "moat-regi..."
	cases := []struct {
		name string
		w, h int
	}{
		{"narrow_80x30", 80, 30},
		{"wide_120x40", 120, 40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := snapshotApp(t, testAppWithMOATItems(t, tc.w, tc.h))
			if !strings.Contains(snap, truncated) {
				t.Errorf("expected scope-column truncation marker %q at %dx%d; got:\n%s",
					truncated, tc.w, tc.h, snap)
			}
			// The Unicode ellipsis would indicate a drift away from the
			// truncate() helper — a different code path is rendering scope.
			if strings.Contains(snap, "moat-regi…") {
				t.Errorf("scope column rendered U+2026 ellipsis at %dx%d; truncate() uses three ASCII dots",
					tc.w, tc.h)
			}
		})
	}
}

// TestResponsive_LibraryNoTruncation_ShortScopes guards the other direction:
// when all row data fits within fixed-width columns, no "..." truncation
// marker should appear in the table body. The default library fixture uses
// "team-rules" / "library" / "my-registry" (all <=12 runes) so the Scope
// column should render without truncation at either size.
//
// This catches a regression where truncate() is called with the wrong
// column width (e.g., 9 instead of 12) and mangles short strings.
func TestResponsive_LibraryNoTruncation_ShortScopes(t *testing.T) {
	cases := []struct {
		name string
		w, h int
	}{
		{"narrow_80x30", 80, 30},
		{"wide_120x40", 120, 40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := snapshotApp(t, testAppWithItemsSize(t, tc.w, tc.h))
			// Look specifically for a truncated fixture scope name. "library"
			// or "team-rules" etc. would look like "librar..." / "team-r..."
			// under a bad column width. Only rows (lines starting with "│ ")
			// are inspected so we don't false-positive on metapanel text.
			for lineNo, line := range strings.Split(snap, "\n") {
				if !strings.HasPrefix(strings.TrimSpace(line), "│") {
					continue
				}
				for _, probe := range []string{"librar...", "team-r...", "my-reg..."} {
					if strings.Contains(line, probe) {
						t.Errorf("row line %d shows %q — short scope names should not truncate at %dx%d:\n%s",
							lineNo+1, probe, tc.w, tc.h, line)
					}
				}
			}
		})
	}
}

// TestResponsive_MOATLibraryScope_CountMatchesFixture asserts the Scope
// column truncates once per row with a long registry name. The MOAT
// fixture contributes three such rows (verified/revoked/private); the
// vanilla row has Source="library" which fits. The metapanel may also
// surface the registry name, so we assert a lower bound rather than
// exact count to stay robust to metapanel text changes.
func TestResponsive_MOATLibraryScope_CountMatchesFixture(t *testing.T) {
	snap := snapshotApp(t, testAppWithMOATItems(t, 120, 40))
	const truncated = "moat-regi..."
	got := strings.Count(snap, truncated)
	const wantAtLeast = 3 // verified + revoked + private rows
	if got < wantAtLeast {
		t.Errorf("expected at least %d %q occurrences (one per MOAT row); got %d\n%s",
			wantAtLeast, truncated, got, snap)
	}
}
