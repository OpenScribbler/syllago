package provider

import (
	"reflect"
	"testing"
)

func TestMonolithicHint(t *testing.T) {
	cases := []struct {
		slug string
		want string
	}{
		{"codex", "Codex prefers per-directory AGENTS.md files; consider installing per directory rather than as a single root file."},
		{"windsurf", "Windsurf has a 6KB limit on this file; the file rules format (.windsurf/rules/) is recommended for non-trivial content."},
		{"claude-code", ""},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			got := MonolithicHint(tc.slug)
			if got != tc.want {
				t.Fatalf("MonolithicHint(%q) = %q, want %q", tc.slug, got, tc.want)
			}
		})
	}
}

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
