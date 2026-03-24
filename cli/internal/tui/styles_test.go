package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestInlineTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		width int
	}{
		{"standard", "Skills (5)", 40},
		{"narrow", "Skills", 20},
		{"very narrow", "Hi", 8},
		{"wide", "Long Section Title Here", 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inlineTitle(tt.title, tt.width, primaryColor)
			w := lipgloss.Width(result)
			if w > tt.width+2 { // small tolerance for ANSI codes
				t.Errorf("inlineTitle(%q, %d) width = %d, want <= %d", tt.title, tt.width, w, tt.width)
			}
			if result == "" {
				t.Error("inlineTitle returned empty string")
			}
		})
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		text     string
		maxWidth int
		wantLen  int // approx
	}{
		{"hello", 10, 5},
		{"hello world", 8, 8},
		{"hi", 2, 2},
		{"", 5, 0},
	}
	for _, tt := range tests {
		result := truncateStr(tt.text, tt.maxWidth)
		if len(result) > tt.maxWidth {
			t.Errorf("truncateStr(%q, %d) = %q (len %d), exceeds max", tt.text, tt.maxWidth, result, len(result))
		}
	}
}
