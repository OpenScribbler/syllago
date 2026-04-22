package tui

import (
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
