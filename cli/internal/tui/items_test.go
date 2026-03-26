package tui

import (
	"testing"
)

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
