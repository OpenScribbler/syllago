package canonical

import (
	"bytes"
	"testing"
)

func TestNormalize_Empty(t *testing.T) {
	got := Normalize(nil)
	want := []byte{'\n'}
	if !bytes.Equal(got, want) {
		t.Fatalf("Normalize(nil) = %q, want %q", got, want)
	}
}

func TestNormalize_Cases(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{"crlf", []byte("a\r\nb\r\n"), []byte("a\nb\n")},
		{"bom", []byte("\xEF\xBB\xBFhello\n"), []byte("hello\n")},
		{"no_trailing_newline", []byte("hello"), []byte("hello\n")},
		{"double_trailing_newline", []byte("hello\n\n"), []byte("hello\n")},
		{"preserve_two_trailing_spaces", []byte("line  \n"), []byte("line  \n")},
		{"preserve_tabs", []byte("\thello\n"), []byte("\thello\n")},
		{"preserve_unicode", []byte("café\n"), []byte("café\n")},
		{"preserve_heading_case", []byte("# FOO\n"), []byte("# FOO\n")},
		{"empty_to_newline", []byte(""), []byte("\n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Normalize(tc.input)
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("Normalize(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
