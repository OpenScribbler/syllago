package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestSearchActivateDeactivate(t *testing.T) {
	t.Parallel()
	m := newSearchModel()

	if m.active {
		t.Fatal("search should start inactive")
	}

	m.activate()
	if !m.active {
		t.Fatal("search should be active after activate()")
	}
	if m.query != "" {
		t.Fatal("query should be empty after activate()")
	}

	m.deactivate()
	if m.active {
		t.Fatal("search should be inactive after deactivate()")
	}
}

func TestSearchActivateClearsQuery(t *testing.T) {
	t.Parallel()
	m := newSearchModel()
	m.query = "leftover"
	m.activate()
	if m.query != "" {
		t.Fatal("activate() should clear existing query")
	}
}

func TestSearchQueryAppendsCharacters(t *testing.T) {
	t.Parallel()
	m := newSearchModel()
	m.activate()

	cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.query != "h" {
		t.Fatalf("expected query 'h', got %q", m.query)
	}
	if cmd == nil {
		t.Fatal("expected a command for query change")
	}
	msg := cmd()
	if qm, ok := msg.(searchQueryMsg); !ok || qm.query != "h" {
		t.Fatalf("expected searchQueryMsg with query 'h', got %T %v", msg, msg)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if m.query != "hi" {
		t.Fatalf("expected query 'hi', got %q", m.query)
	}
}

func TestSearchBackspaceRemoves(t *testing.T) {
	t.Parallel()
	m := newSearchModel()
	m.activate()
	m.query = "abc"

	cmd := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.query != "ab" {
		t.Fatalf("expected query 'ab', got %q", m.query)
	}
	msg := cmd()
	if qm, ok := msg.(searchQueryMsg); !ok || qm.query != "ab" {
		t.Fatalf("expected searchQueryMsg with query 'ab', got %T %v", msg, msg)
	}
}

func TestSearchBackspaceOnEmpty(t *testing.T) {
	t.Parallel()
	m := newSearchModel()
	m.activate()

	cmd := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.query != "" {
		t.Fatalf("expected empty query, got %q", m.query)
	}
	if cmd == nil {
		t.Fatal("expected command even on empty backspace")
	}
}

func TestSearchEscCancels(t *testing.T) {
	t.Parallel()
	m := newSearchModel()
	m.activate()
	m.query = "test"

	cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.active {
		t.Fatal("search should be inactive after Esc")
	}
	if cmd == nil {
		t.Fatal("expected a command for cancel")
	}
	msg := cmd()
	if _, ok := msg.(searchCancelMsg); !ok {
		t.Fatalf("expected searchCancelMsg, got %T", msg)
	}
}

func TestSearchEnterConfirms(t *testing.T) {
	t.Parallel()
	m := newSearchModel()
	m.activate()
	m.query = "myquery"

	cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.active {
		t.Fatal("search should be inactive after Enter")
	}
	if cmd == nil {
		t.Fatal("expected a command for confirm")
	}
	msg := cmd()
	cm, ok := msg.(searchConfirmMsg)
	if !ok {
		t.Fatalf("expected searchConfirmMsg, got %T", msg)
	}
	if cm.query != "myquery" {
		t.Fatalf("expected query 'myquery', got %q", cm.query)
	}
}

func TestSearchInactivePassesThrough(t *testing.T) {
	t.Parallel()
	m := newSearchModel()

	cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatal("inactive search should return nil cmd")
	}
	if m.query != "" {
		t.Fatal("inactive search should not modify query")
	}
}

func TestSearchViewActive(t *testing.T) {
	t.Parallel()
	m := newSearchModel()
	m.activate()
	m.query = "hello"

	v := m.View()
	if v == "" {
		t.Fatal("active search should render non-empty view")
	}
	if !strings.Contains(v, "/") {
		t.Error("view should contain '/' prefix")
	}
	if !strings.Contains(v, "hello") {
		t.Error("view should contain query text")
	}
	// Block cursor character
	if !strings.Contains(v, "\u2588") {
		t.Error("view should contain cursor character")
	}
}

func TestSearchViewInactive(t *testing.T) {
	t.Parallel()
	m := newSearchModel()

	v := m.View()
	if v != "" {
		t.Fatalf("inactive search should render empty string, got %q", v)
	}
}

func TestFilterItems(t *testing.T) {
	t.Parallel()

	items := []catalog.ContentItem{
		{Name: "my-rule"},
		{Name: "Other-Skill"},
		{Name: "another-rule"},
		{Name: "test-hook"},
	}

	tests := []struct {
		name     string
		query    string
		expected int
		names    []string
	}{
		{
			name:     "empty query returns all",
			query:    "",
			expected: 4,
		},
		{
			name:     "case insensitive match",
			query:    "RULE",
			expected: 2,
			names:    []string{"my-rule", "another-rule"},
		},
		{
			name:     "partial match",
			query:    "other",
			expected: 2,
			names:    []string{"Other-Skill", "another-rule"},
		},
		{
			name:     "no match returns empty",
			query:    "zzz",
			expected: 0,
		},
		{
			name:     "exact match",
			query:    "test-hook",
			expected: 1,
			names:    []string{"test-hook"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := filterItems(items, tt.query)
			if len(result) != tt.expected {
				t.Fatalf("expected %d results, got %d", tt.expected, len(result))
			}
			if tt.names != nil {
				for i, name := range tt.names {
					if result[i].Name != name {
						t.Errorf("result[%d] = %q, want %q", i, result[i].Name, name)
					}
				}
			}
		})
	}
}

func TestFilterItemsEmptySlice(t *testing.T) {
	t.Parallel()
	result := filterItems(nil, "test")
	if len(result) != 0 {
		t.Fatalf("expected 0 results for nil input, got %d", len(result))
	}
}
