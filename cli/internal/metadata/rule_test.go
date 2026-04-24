package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRuleMetadata_YAMLRoundtrip(t *testing.T) {
	captured := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	added := time.Date(2026, 4, 24, 12, 5, 0, 0, time.UTC)
	written := time.Date(2026, 4, 24, 12, 5, 30, 0, time.UTC)
	orig := RuleMetadata{
		FormatVersion: 1,
		ID:            "coding-style",
		Name:          "Coding Style",
		Description:   "Project conventions",
		Type:          "rule",
		AddedAt:       added,
		AddedBy:       "syllago",
		Source: RuleSource{
			Provider:         "claude-code",
			Scope:            "project",
			Path:             "/home/user/proj/CLAUDE.md",
			Format:           "claude-code-monolithic",
			Filename:         "CLAUDE.md",
			Hash:             "sha256:" + stringRepeat("a", 64),
			SplitMethod:      "h2",
			SplitFromSection: "## Coding Style",
			CapturedAt:       captured,
		},
		Versions: []RuleVersionEntry{
			{
				Hash:      "sha256:" + stringRepeat("b", 64),
				WrittenAt: written,
			},
		},
		CurrentVersion: "sha256:" + stringRepeat("b", 64),
	}
	data, err := yaml.Marshal(&orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back RuleMetadata
	if err := yaml.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data2, err := yaml.Marshal(&back)
	if err != nil {
		t.Fatalf("remarshal: %v", err)
	}
	if string(data) != string(data2) {
		t.Fatalf("roundtrip not byte-equal:\nfirst:\n%s\nsecond:\n%s", data, data2)
	}
}

// stringRepeat is a tiny helper to avoid importing strings here.
func stringRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func TestLoadRuleMetadata_RejectsMalformedHash(t *testing.T) {
	cases := []struct {
		name string
		hash string
	}{
		{"missing_algo_prefix", strings.Repeat("a", 64)},
		{"wrong_hex_length", "sha256:abc"},
		{"extra_characters", "sha256:" + strings.Repeat("a", 65)},
		{"colon_to_dash", "sha256-" + strings.Repeat("a", 64)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, ".syllago.yaml")
			m := RuleMetadata{
				FormatVersion: 1,
				ID:            "x",
				Name:          "x",
				Type:          "rule",
				AddedAt:       time.Now(),
				Source: RuleSource{
					Provider:    "claude-code",
					Scope:       "project",
					Path:        "/tmp/CLAUDE.md",
					Format:      "claude-code-monolithic",
					Filename:    "CLAUDE.md",
					Hash:        "sha256:" + strings.Repeat("a", 64),
					SplitMethod: "h2",
					CapturedAt:  time.Now(),
				},
				Versions: []RuleVersionEntry{
					{Hash: tc.hash, WrittenAt: time.Now()},
				},
				CurrentVersion: tc.hash,
			}
			data, err := yaml.Marshal(&m)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			_, err = LoadRuleMetadata(path)
			if err == nil {
				t.Fatalf("expected error for hash %q", tc.hash)
			}
			if !strings.Contains(err.Error(), "invalid hash format") {
				t.Fatalf("error = %q, want substring %q", err, "invalid hash format")
			}
		})
	}
}

func TestLoadRuleMetadata_CurrentVersionMustExist(t *testing.T) {
	validHashA := "sha256:" + strings.Repeat("a", 64)
	validHashB := "sha256:" + strings.Repeat("b", 64)
	cases := []struct {
		name           string
		versions       []RuleVersionEntry
		currentVersion string
	}{
		{
			name: "missing_hash_reference",
			versions: []RuleVersionEntry{
				{Hash: validHashA, WrittenAt: time.Now()},
			},
			currentVersion: validHashB,
		},
		{
			name:           "empty_versions_with_current",
			versions:       nil,
			currentVersion: validHashA,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, ".syllago.yaml")
			m := RuleMetadata{
				FormatVersion: 1,
				ID:            "x",
				Name:          "x",
				Type:          "rule",
				AddedAt:       time.Now(),
				Source: RuleSource{
					Provider:    "claude-code",
					Scope:       "project",
					Path:        "/tmp/CLAUDE.md",
					Format:      "claude-code-monolithic",
					Filename:    "CLAUDE.md",
					Hash:        validHashA,
					SplitMethod: "h2",
					CapturedAt:  time.Now(),
				},
				Versions:       tc.versions,
				CurrentVersion: tc.currentVersion,
			}
			data, err := yaml.Marshal(&m)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			_, err = LoadRuleMetadata(path)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), "current_version references missing hash") {
				t.Fatalf("error = %q, want substring %q", err, "current_version references missing hash")
			}
		})
	}
}
