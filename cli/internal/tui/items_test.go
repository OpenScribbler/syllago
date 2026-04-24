package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// --- itemsModel navigation coverage ---

func makeItems(n int) []catalog.ContentItem {
	items := make([]catalog.ContentItem, n)
	for i := range items {
		items[i] = catalog.ContentItem{Name: "item", Type: catalog.Rules}
	}
	return items
}

func TestItemsModel_CursorDown_Wraps(t *testing.T) {
	t.Parallel()
	m := newItemsModel(makeItems(3), false)
	m.SetSize(40, 10)
	m.cursor = 2
	m.CursorDown() // wraps to 0
	if m.cursor != 0 {
		t.Errorf("expected wrap to 0, got %d", m.cursor)
	}
	if m.offset != 0 {
		t.Errorf("expected offset reset to 0 after wrap, got %d", m.offset)
	}
}

func TestItemsModel_CursorDown_OffsetAdvances(t *testing.T) {
	t.Parallel()
	m := newItemsModel(makeItems(20), false)
	m.SetSize(40, 5)
	for i := 0; i < 6; i++ {
		m.CursorDown()
	}
	if m.offset == 0 {
		t.Errorf("expected offset to advance when cursor exceeds visible window, got %d", m.offset)
	}
}

func TestItemsModel_CursorDown_Empty(t *testing.T) {
	t.Parallel()
	m := newItemsModel(nil, false)
	m.CursorDown() // no panic
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 for empty list, got %d", m.cursor)
	}
}

func TestItemsModel_PageUp(t *testing.T) {
	t.Parallel()
	m := newItemsModel(makeItems(30), false)
	m.SetSize(40, 5)
	m.cursor = 20
	m.offset = 15
	m.PageUp()
	if m.cursor != 15 {
		t.Errorf("expected cursor=15 after PageUp, got %d", m.cursor)
	}
	if m.offset != 10 {
		t.Errorf("expected offset=10 after PageUp, got %d", m.offset)
	}
}

func TestItemsModel_PageUp_ClampsAtZero(t *testing.T) {
	t.Parallel()
	m := newItemsModel(makeItems(10), false)
	m.SetSize(40, 5)
	m.cursor = 2
	m.offset = 0
	m.PageUp()
	if m.cursor != 0 {
		t.Errorf("expected cursor clamped to 0, got %d", m.cursor)
	}
}

func TestItemsModel_PageDown(t *testing.T) {
	t.Parallel()
	m := newItemsModel(makeItems(30), false)
	m.SetSize(40, 5)
	m.cursor = 0
	m.offset = 0
	m.PageDown()
	if m.cursor != 5 {
		t.Errorf("expected cursor=5 after PageDown, got %d", m.cursor)
	}
	if m.offset != 5 {
		t.Errorf("expected offset=5 after PageDown, got %d", m.offset)
	}
}

func TestItemsModel_PageDown_ClampsAtEnd(t *testing.T) {
	t.Parallel()
	m := newItemsModel(makeItems(8), false)
	m.SetSize(40, 5)
	m.cursor = 6
	m.offset = 3
	m.PageDown()
	if m.cursor != 7 {
		t.Errorf("expected cursor clamped to 7 (len-1), got %d", m.cursor)
	}
}

func TestWordWrap(t *testing.T) {
	tests := []struct {
		name  string
		input string
		maxW  int
		want  []string
	}{
		{
			name:  "fits on one line",
			input: "short text",
			maxW:  20,
			want:  []string{"short text"},
		},
		{
			name:  "wraps at word boundary",
			input: "the quick brown fox jumps over",
			maxW:  16,
			want:  []string{"the quick brown", "fox jumps over"},
		},
		{
			name:  "long word force-broken",
			input: "hello superlongwordthatexceedslimit end",
			maxW:  15,
			want:  []string{"hello", "superlongwordth", "atexceedslimit", "end"},
		},
		{
			name:  "empty string",
			input: "",
			maxW:  20,
			want:  []string{""},
		},
		{
			name:  "exact fit",
			input: "exactly",
			maxW:  7,
			want:  []string{"exactly"},
		},
		{
			name:  "multiple spaces collapsed",
			input: "hello   world",
			maxW:  20,
			want:  []string{"hello world"},
		},
		{
			name:  "wraps three lines",
			input: "A curated collection of AI coding tool content for Python web development workflows",
			maxW:  30,
			want:  []string{"A curated collection of AI", "coding tool content for Python", "web development workflows"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wordWrap(tt.input, tt.maxW)
			if len(got) != len(tt.want) {
				t.Errorf("wordWrap(%q, %d) = %v (len %d), want %v (len %d)", tt.input, tt.maxW, got, len(got), tt.want, len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
