package rulestore

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func newTestMeta(t *testing.T) metadata.RuleMetadata {
	t.Helper()
	return metadata.RuleMetadata{
		ID:      "claude-code/coding-style",
		Name:    "Coding style",
		AddedAt: time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
		Source: metadata.RuleSource{
			Provider:    "claude-code",
			Scope:       "project",
			Path:        "CLAUDE.md",
			Format:      "claude-code",
			Filename:    "CLAUDE.md",
			Hash:        "sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			SplitMethod: "h2",
			CapturedAt:  time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
		},
	}
}

func TestLoadRule_RoundTrip(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	body := []byte("# Coding style\n\nUse tabs.\n")
	meta := newTestMeta(t)
	if err := WriteRule(tmp, "claude-code", "coding-style", meta, body); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}
	dir := filepath.Join(tmp, "claude-code", "coding-style")

	loaded, err := LoadRule(dir)
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}

	wantHash := HashBody(body)
	if loaded.Meta.CurrentVersion != wantHash {
		t.Errorf("Meta.CurrentVersion: got %q, want %q", loaded.Meta.CurrentVersion, wantHash)
	}
	if len(loaded.Meta.Versions) != 1 {
		t.Fatalf("versions: got %d, want 1", len(loaded.Meta.Versions))
	}
	if loaded.Meta.Versions[0].Hash != wantHash {
		t.Errorf("versions[0].Hash: got %q, want %q", loaded.Meta.Versions[0].Hash, wantHash)
	}

	// History map keyed on canonical hash, byte-equal to canonical body.
	if len(loaded.History) != 1 {
		t.Fatalf("History: got %d entries, want 1", len(loaded.History))
	}
	got, ok := loaded.History[wantHash]
	if !ok {
		t.Fatalf("History: missing key %q (have %v)", wantHash, keysOf(loaded.History))
	}
	if !bytes.Equal(got, canonical.Normalize(body)) {
		t.Errorf("History body: got %q, want %q", got, canonical.Normalize(body))
	}

	if loaded.Dir != dir {
		t.Errorf("Dir: got %q, want %q", loaded.Dir, dir)
	}
}

func TestLoadRule_MissingHistoryFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	body := []byte("# Coding style\n\nUse tabs.\n")
	meta := newTestMeta(t)
	if err := WriteRule(tmp, "claude-code", "coding-style", meta, body); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}
	dir := filepath.Join(tmp, "claude-code", "coding-style")
	// Delete the single history file, keeping the versions[] entry intact.
	historyPath := filepath.Join(dir, ".history", hashToFilename(HashBody(body)))
	if err := os.Remove(historyPath); err != nil {
		t.Fatalf("removing %s: %v", historyPath, err)
	}
	_, err := LoadRule(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing history file") {
		t.Errorf("error %q does not contain %q", err.Error(), "missing history file")
	}
}

func keysOf(m map[string][]byte) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
