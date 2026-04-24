package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// tableWithItems returns a tableModel populated with n fabricated items,
// sized wide enough that the search bar and all columns render.
func tableWithItems(t *testing.T, n int) tableModel {
	t.Helper()
	items := make([]catalog.ContentItem, n)
	for i := 0; i < n; i++ {
		items[i] = catalog.ContentItem{
			Name:        string(rune('a'+i)) + "-item",
			Type:        catalog.Skills,
			Source:      "lib",
			Files:       []string{"SKILL.md"},
			Description: "desc",
		}
	}
	tbl := newTableModel(items, nil, "")
	tbl.SetSize(120, 20)
	return tbl
}

func TestProviderAbbrev_KnownSlugs(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"claude-code": "CC",
		"gemini-cli":  "GC",
		"cursor":      "Cu",
		"copilot":     "Co",
		"windsurf":    "WS",
		"kiro":        "Ki",
		"cline":       "Cl",
		"roo-code":    "RC",
		"amp":         "Am",
		"opencode":    "OC",
		"zed":         "Zd",
	}
	for slug, want := range cases {
		if got := providerAbbrev(slug); got != want {
			t.Errorf("providerAbbrev(%q) = %q, want %q", slug, got, want)
		}
	}
}

func TestProviderAbbrev_UnknownSlug(t *testing.T) {
	t.Parallel()
	if got := providerAbbrev("factory-droid"); got != "FA" {
		t.Errorf("providerAbbrev long unknown: got %q, want FA", got)
	}
	if got := providerAbbrev("x"); got != "x" {
		t.Errorf("providerAbbrev short unknown: got %q, want x", got)
	}
	if got := providerAbbrev(""); got != "" {
		t.Errorf("providerAbbrev empty: got %q, want \"\"", got)
	}
}

func TestAbbrevToSlug_RoundTrip(t *testing.T) {
	t.Parallel()
	slugs := []string{
		"claude-code", "gemini-cli", "cursor", "copilot", "windsurf",
		"kiro", "cline", "roo-code", "amp", "opencode", "zed",
	}
	for _, slug := range slugs {
		abbr := providerAbbrev(slug)
		if got := abbrevToSlug(abbr); got != slug {
			t.Errorf("abbrevToSlug(providerAbbrev(%q))=%q, want %q", slug, got, slug)
		}
	}
}

func TestAbbrevToSlug_Unknown(t *testing.T) {
	t.Parallel()
	// Unknown abbreviations are lowercased.
	if got := abbrevToSlug("XX"); got != "xx" {
		t.Errorf("abbrevToSlug(XX) = %q, want xx", got)
	}
}

func TestProviderFullName_KnownSlugs(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"claude-code": "Claude Code",
		"gemini-cli":  "Gemini CLI",
		"cursor":      "Cursor",
		"copilot":     "Copilot",
		"windsurf":    "Windsurf",
		"kiro":        "Kiro",
		"cline":       "Cline",
		"roo-code":    "Roo Code",
		"amp":         "Amp",
		"opencode":    "OpenCode",
		"zed":         "Zed",
	}
	for slug, want := range cases {
		if got := providerFullName(slug); got != want {
			t.Errorf("providerFullName(%q) = %q, want %q", slug, got, want)
		}
	}
}

func TestProviderFullName_UnknownReturnsSlug(t *testing.T) {
	t.Parallel()
	if got := providerFullName("custom-provider"); got != "custom-provider" {
		t.Errorf("providerFullName unknown: got %q, want custom-provider", got)
	}
}

func TestTable_PageDownAdvancesCursor(t *testing.T) {
	t.Parallel()
	tbl := tableWithItems(t, 50)
	startCursor := tbl.cursor
	tbl.PageDown()
	if tbl.cursor <= startCursor {
		t.Errorf("PageDown did not advance cursor: start=%d after=%d", startCursor, tbl.cursor)
	}
	if tbl.cursor >= len(tbl.items) {
		t.Errorf("PageDown moved cursor past last item: cursor=%d len=%d", tbl.cursor, len(tbl.items))
	}
	if tbl.offset < 0 {
		t.Errorf("PageDown produced negative offset: %d", tbl.offset)
	}
}

func TestTable_PageUpRestoresCursor(t *testing.T) {
	t.Parallel()
	tbl := tableWithItems(t, 50)
	tbl.PageDown()
	tbl.PageDown()
	tbl.PageUp()
	// After one down-down-up we should be strictly less than two pages down.
	if tbl.cursor < 0 || tbl.cursor >= len(tbl.items) {
		t.Errorf("PageUp left invalid cursor: %d", tbl.cursor)
	}
	if tbl.offset < 0 {
		t.Errorf("PageUp left negative offset: %d", tbl.offset)
	}
}

func TestTable_PageUpAtTopIsNoop(t *testing.T) {
	t.Parallel()
	tbl := tableWithItems(t, 50)
	tbl.PageUp()
	if tbl.cursor != 0 {
		t.Errorf("PageUp at top should keep cursor=0, got %d", tbl.cursor)
	}
	if tbl.offset != 0 {
		t.Errorf("PageUp at top should keep offset=0, got %d", tbl.offset)
	}
}

func TestTable_SortByColumn_ToggleDirection(t *testing.T) {
	t.Parallel()
	tbl := tableWithItems(t, 5)
	// Default is sortByName, sortAsc=true.
	if tbl.sortCol != sortByName || !tbl.sortAsc {
		t.Fatalf("initial sort state wrong: col=%d asc=%v", tbl.sortCol, tbl.sortAsc)
	}

	tbl.SortByColumn(sortByName)
	if tbl.sortAsc {
		t.Errorf("SortByColumn on same col should toggle to desc, got asc=true")
	}

	tbl.SortByColumn(sortByName)
	if !tbl.sortAsc {
		t.Errorf("SortByColumn twice on same col should restore asc=true, got false")
	}
}

func TestTable_SortByColumn_SwitchKeepsDirection(t *testing.T) {
	t.Parallel()
	tbl := tableWithItems(t, 5)
	tbl.sortAsc = false
	tbl.SortByColumn(sortByType)
	if tbl.sortCol != sortByType {
		t.Errorf("expected sortCol=sortByType, got %d", tbl.sortCol)
	}
	if tbl.sortAsc {
		t.Errorf("SortByColumn to new col should preserve sortAsc=false")
	}
}

func TestTable_SearchBackspace_PopsLastRune(t *testing.T) {
	t.Parallel()
	tbl := tableWithItems(t, 5)
	tbl.StartSearch()
	tbl.SearchType('a')
	tbl.SearchType('b')
	tbl.SearchType('c')
	if tbl.searchQuery != "abc" {
		t.Fatalf("expected searchQuery=abc, got %q", tbl.searchQuery)
	}

	tbl.SearchBackspace()
	if tbl.searchQuery != "ab" {
		t.Errorf("expected ab after backspace, got %q", tbl.searchQuery)
	}
	tbl.SearchBackspace()
	tbl.SearchBackspace()
	if tbl.searchQuery != "" {
		t.Errorf("expected empty after 3 backspaces, got %q", tbl.searchQuery)
	}
	// Backspace on empty is a safe no-op.
	tbl.SearchBackspace()
	if tbl.searchQuery != "" {
		t.Errorf("expected empty after backspace on empty, got %q", tbl.searchQuery)
	}
}

func TestTable_SearchBackspace_Unicode(t *testing.T) {
	t.Parallel()
	tbl := tableWithItems(t, 5)
	tbl.StartSearch()
	tbl.SearchType('é')
	tbl.SearchType('中')
	tbl.SearchBackspace()
	if tbl.searchQuery != "é" {
		t.Errorf("expected \"é\" after popping CJK rune, got %q", tbl.searchQuery)
	}
}

func TestTable_RenderSearchBar_Active(t *testing.T) {
	t.Parallel()
	tbl := tableWithItems(t, 3)
	tbl.StartSearch()
	tbl.SearchType('a')
	view := tbl.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "/ a") {
		t.Errorf("expected search prompt \"/ a\" in view, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "esc cancel") {
		t.Errorf("expected active search hints, got:\n%s", stripped)
	}
}

func TestTable_RenderSearchBar_Confirmed(t *testing.T) {
	t.Parallel()
	tbl := tableWithItems(t, 3)
	tbl.StartSearch()
	tbl.SearchType('x')
	tbl.SearchConfirm()
	if tbl.searching {
		t.Fatal("expected searching=false after SearchConfirm")
	}
	view := tbl.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "esc clear") {
		t.Errorf("expected \"esc clear\" hint for inactive-but-queried search, got:\n%s", stripped)
	}
}

func TestMatchesInstalled(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		installed string
		query     string
		want      bool
	}{
		{"empty installed", "", "cc", false},
		{"dash-dash sentinel", "--", "cc", false},
		{"direct abbrev match lowercase", "CC,GC", "cc", true},
		{"direct abbrev multi match", "CC,GC", "gc", true},
		{"no match empty query against installed", "CC", "xxx", false},
		{"full name match via expansion", "CC", "claude code", true},
		{"full name partial via expansion", "CC", "claude", true},
		{"full name gemini via expansion", "GC", "gemini", true},
		{"no match against unrelated name", "CC", "cursor", false},
		{"match through comma list expansion", "CC,Cu", "cursor", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := matchesInstalled(tc.installed, tc.query); got != tc.want {
				t.Errorf("matchesInstalled(%q, %q) = %v, want %v", tc.installed, tc.query, got, tc.want)
			}
		})
	}
}

func TestComputeLoadoutDetail_WithProviderAndItems(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	yaml := `kind: loadout
version: 1
name: my-loadout
description: test loadout
provider: claude-code
skills:
  - s1
  - s2
rules:
  - r1
hooks:
  - h1
agents:
  - a1
mcp:
  - m1
commands:
  - c1
`
	if err := os.WriteFile(filepath.Join(dir, "loadout.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write loadout.yaml: %v", err)
	}
	item := catalog.ContentItem{Path: dir, Type: catalog.Loadouts, Name: "my-loadout"}
	got := computeLoadoutDetail(item)
	if !strings.Contains(got, "Target: claude-code") {
		t.Errorf("expected target in detail, got %q", got)
	}
	for _, want := range []string{"2 skills", "1 rules", "1 hooks", "1 agents", "1 mcp", "1 commands"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in detail, got %q", want, got)
		}
	}
}

func TestComputeLoadoutDetail_MissingFileReturnsEmpty(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{Path: t.TempDir(), Type: catalog.Loadouts}
	if got := computeLoadoutDetail(item); got != "" {
		t.Errorf("expected empty string for missing loadout.yaml, got %q", got)
	}
}

func TestComputeLoadoutDetail_EmptyManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	yaml := `kind: loadout
version: 1
name: empty-loadout
description: no items
`
	if err := os.WriteFile(filepath.Join(dir, "loadout.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write loadout.yaml: %v", err)
	}
	item := catalog.ContentItem{Path: dir, Type: catalog.Loadouts}
	got := computeLoadoutDetail(item)
	if got != "" {
		t.Errorf("expected empty detail for empty manifest, got %q", got)
	}
}

func TestComputeTypeDetail_Dispatch(t *testing.T) {
	t.Parallel()
	// Non-Hooks/MCP/Loadouts returns empty.
	if got := computeTypeDetail(catalog.ContentItem{Type: catalog.Skills}); got != "" {
		t.Errorf("expected empty for Skills, got %q", got)
	}
	// Loadouts dispatches to computeLoadoutDetail (missing file -> empty).
	if got := computeTypeDetail(catalog.ContentItem{Type: catalog.Loadouts, Path: t.TempDir()}); got != "" {
		t.Errorf("expected empty for missing loadout, got %q", got)
	}
}

func TestTable_RenderSearchBar_ShowsMatchCount(t *testing.T) {
	t.Parallel()
	tbl := tableWithItems(t, 10)
	tbl.StartSearch()
	tbl.SearchType('a') // "a-item" matches only row 0
	view := tbl.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "(1/10)") {
		t.Errorf("expected match count (1/10) in search bar, got:\n%s", stripped)
	}
}
