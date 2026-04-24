package rulestore

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
)

func TestAppendVersion(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	bodyA := []byte("# Coding style\n\nUse tabs.\n")
	meta := newTestMeta(t)
	if err := WriteRule(tmp, "claude-code", "coding-style", meta, bodyA); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}
	dir := filepath.Join(tmp, "claude-code", "coding-style")

	bodyB := []byte("# Coding style\n\nUse spaces.\n")
	if err := AppendVersion(dir, bodyB); err != nil {
		t.Fatalf("AppendVersion: %v", err)
	}

	loaded, err := LoadRule(dir)
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}

	hashA := HashBody(bodyA)
	hashB := HashBody(bodyB)
	if len(loaded.Meta.Versions) != 2 {
		t.Fatalf("versions: got %d, want 2", len(loaded.Meta.Versions))
	}
	if loaded.Meta.Versions[0].Hash != hashA {
		t.Errorf("versions[0]: got %q, want %q", loaded.Meta.Versions[0].Hash, hashA)
	}
	if loaded.Meta.Versions[1].Hash != hashB {
		t.Errorf("versions[1]: got %q, want %q", loaded.Meta.Versions[1].Hash, hashB)
	}
	if loaded.Meta.CurrentVersion != hashB {
		t.Errorf("current_version: got %q, want %q", loaded.Meta.CurrentVersion, hashB)
	}

	// rule.md equals canonical(bodyB).
	ruleMD, err := os.ReadFile(filepath.Join(dir, "rule.md"))
	if err != nil {
		t.Fatalf("reading rule.md: %v", err)
	}
	if !bytes.Equal(ruleMD, canonical.Normalize(bodyB)) {
		t.Errorf("rule.md: got %q, want %q", ruleMD, canonical.Normalize(bodyB))
	}

	// Both history files exist.
	for _, h := range []string{hashA, hashB} {
		p := filepath.Join(dir, ".history", hashToFilename(h))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected history file %s: %v", p, err)
		}
	}
}

func TestAppendVersion_DedupByHash(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	body := []byte("# Coding style\n\nUse tabs.\n")
	meta := newTestMeta(t)
	if err := WriteRule(tmp, "claude-code", "coding-style", meta, body); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}
	dir := filepath.Join(tmp, "claude-code", "coding-style")

	// Re-append the same body — should dedup by hash.
	if err := AppendVersion(dir, body); err != nil {
		t.Fatalf("AppendVersion: %v", err)
	}
	loaded, err := LoadRule(dir)
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}
	if len(loaded.Meta.Versions) != 1 {
		t.Errorf("versions: got %d, want 1 (dedup by hash)", len(loaded.Meta.Versions))
	}
}
