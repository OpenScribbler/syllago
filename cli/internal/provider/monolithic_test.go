package provider

import (
	"reflect"
	"testing"
)

func TestMonolithicFilenames(t *testing.T) {
	cases := []struct {
		slug string
		want []string
	}{
		{"claude-code", []string{"CLAUDE.md"}},
		{"codex", []string{"AGENTS.md"}},
		{"gemini-cli", []string{"GEMINI.md"}},
		{"cursor", []string{".cursorrules"}},
		{"cline", []string{".clinerules"}},
		{"windsurf", []string{".windsurfrules"}},
		{"unknown", nil},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			got := MonolithicFilenames(tc.slug)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("MonolithicFilenames(%q) = %v, want %v", tc.slug, got, tc.want)
			}
		})
	}
}
