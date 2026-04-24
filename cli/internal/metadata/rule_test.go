package metadata

import (
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
