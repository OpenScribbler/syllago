package rulestore

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRule_WithSource(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	body := []byte("# Coding style\n\nUse tabs.\n")
	sourceBytes := []byte("# CLAUDE.md\n\n## Coding style\n\nUse tabs.\n")
	meta := newTestMeta(t)

	if err := WriteRuleWithSource(tmp, "claude-code", "coding-style", meta, body, "CLAUDE.md", sourceBytes); err != nil {
		t.Fatalf("WriteRuleWithSource: %v", err)
	}

	dir := filepath.Join(tmp, "claude-code", "coding-style")
	// Source file captured byte-equal.
	got, err := os.ReadFile(filepath.Join(dir, ".source", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading .source/CLAUDE.md: %v", err)
	}
	if !bytes.Equal(got, sourceBytes) {
		t.Errorf(".source bytes: got %q, want %q", got, sourceBytes)
	}

	// Rule still loadable with all invariants intact.
	loaded, err := LoadRule(dir)
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}
	if loaded.Meta.CurrentVersion != HashBody(body) {
		t.Errorf("CurrentVersion: got %q, want %q", loaded.Meta.CurrentVersion, HashBody(body))
	}
}
