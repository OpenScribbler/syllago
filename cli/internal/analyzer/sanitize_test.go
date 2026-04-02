package analyzer

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestSanitizeItem_StripsControlChars(t *testing.T) {
	t.Parallel()
	item := &DetectedItem{
		Name:        "skill\x1b[31mred\x1b[0m",
		Description: "desc\x00with\x07bells",
		Path:        "skills/foo\x01bar/SKILL.md",
		Provider:    "claude-code",
		Type:        catalog.Skills,
		Confidence:  0.90,
	}
	SanitizeItem(item)
	if strings.ContainsAny(item.Name, "\x00\x01\x07\x1b") {
		t.Errorf("Name still contains control chars: %q", item.Name)
	}
	if strings.ContainsAny(item.Description, "\x00\x01\x07\x1b") {
		t.Errorf("Description still contains control chars: %q", item.Description)
	}
	if strings.ContainsAny(item.Path, "\x00\x01") {
		t.Errorf("Path still contains control chars: %q", item.Path)
	}
}

func TestSanitizeItem_TruncatesLongFields(t *testing.T) {
	t.Parallel()
	item := &DetectedItem{
		Name:        strings.Repeat("a", 200),
		Description: strings.Repeat("b", 500),
		Path:        strings.Repeat("c", 400),
		Type:        catalog.Skills,
		Confidence:  0.90,
	}
	SanitizeItem(item)
	if len([]rune(item.Name)) > 80 {
		t.Errorf("Name not truncated: len=%d", len(item.Name))
	}
	if !strings.HasSuffix(item.Name, "…") {
		t.Errorf("Name missing ellipsis: %q", item.Name)
	}
	if len([]rune(item.Description)) > 200 {
		t.Errorf("Description not truncated: len=%d", len(item.Description))
	}
	if len([]rune(item.Path)) > 256 {
		t.Errorf("Path not truncated: len=%d", len(item.Path))
	}
}

func TestSanitizeItem_ScriptSlices(t *testing.T) {
	t.Parallel()
	item := &DetectedItem{
		Type:       catalog.Hooks,
		Confidence: 0.90,
		Scripts:    []string{"hooks/validate.sh\x1b[", strings.Repeat("x", 300)},
		References: []string{"ref\x00bad"},
		Providers:  []string{"provider\x07path"},
	}
	SanitizeItem(item)
	for i, s := range item.Scripts {
		if strings.ContainsAny(s, "\x00\x01\x07\x1b") {
			t.Errorf("Scripts[%d] contains control chars: %q", i, s)
		}
		if len([]rune(s)) > 256 {
			t.Errorf("Scripts[%d] not truncated", i)
		}
	}
	for i, s := range item.References {
		if strings.ContainsAny(s, "\x00\x01\x07\x1b") {
			t.Errorf("References[%d] contains control chars: %q", i, s)
		}
	}
	for i, s := range item.Providers {
		if strings.ContainsAny(s, "\x00\x01\x07\x1b") {
			t.Errorf("Providers[%d] contains control chars: %q", i, s)
		}
	}
}

func TestSanitizeItem_ControlCharsInStructuredFields(t *testing.T) {
	t.Parallel()
	item := &DetectedItem{
		Name: "legit\x1b[0m\x00injected",
		Path: "skills/foo\x00bar",
		Type: catalog.Skills,
	}
	SanitizeItem(item)
	if strings.ContainsAny(item.Name, "\x00\x1b") {
		t.Errorf("Name contains control chars after sanitize: %q", item.Name)
	}
	if strings.Contains(item.Path, "\x00") {
		t.Errorf("Path contains null byte after sanitize: %q", item.Path)
	}
}

func TestSanitizeItem_PreservesTabsAndNewlines(t *testing.T) {
	t.Parallel()
	item := &DetectedItem{
		Name:        "name\twith\ttabs",
		Description: "desc\nwith\nnewlines",
		Type:        catalog.Skills,
	}
	SanitizeItem(item)
	if !strings.Contains(item.Name, "\t") {
		t.Errorf("tabs should be preserved: %q", item.Name)
	}
	if !strings.Contains(item.Description, "\n") {
		t.Errorf("newlines should be preserved: %q", item.Description)
	}
}

func TestSanitizeHex(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"clean hex", "abcdef0123456789", "abcdef0123456789"},
		{"mixed case hex", "ABCDEFabcdef", "ABCDEFabcdef"},
		{"non-hex chars stripped", "abc-xyz-123", "abc123"},
		{"long hash truncated", strings.Repeat("a", 100), strings.Repeat("a", 64)},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeHex(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeHex(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizeItem_EmptyItem(t *testing.T) {
	t.Parallel()
	item := &DetectedItem{}
	SanitizeItem(item) // should not panic
	if item.Name != "" || item.Path != "" {
		t.Errorf("empty fields should stay empty")
	}
}
