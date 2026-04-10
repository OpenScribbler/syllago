package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestSanitizeExtractedString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trailing newline", "hello\n", "hello"},
		{"trailing crlf", "hello\r\n", "hello"},
		{"leading brace", "{key: val}", "%7Bkey: val}"},
		{"leading bracket", "[1,2,3]", "%5B1,2,3]"},
		{"leading colon", ": value", "%3A value"},
		{"leading hash", "# comment", "%23 comment"},
		{"indented leading brace", "  {key: val}", "  %7Bkey: val}"},
		{"normal string", "PreToolUse", "PreToolUse"},
		{"empty string", "", ""},
		{"long string over 512", string(make([]byte, 600)), string(make([]byte, 500)) + " [truncated]"},
		{"leading bang", "! value", "%21 value"},
		{"interior brace is safe", "foo {bar}", "foo {bar}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capmon.SanitizeExtractedString(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeExtractedString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
