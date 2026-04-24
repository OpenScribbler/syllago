package rulestore

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestWriteRule_CreatesLayout(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	body := []byte("# Coding style\n\nUse tabs.\n")
	meta := metadata.RuleMetadata{
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

	if err := WriteRule(tmp, "claude-code", "coding-style", meta, body); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}

	dir := filepath.Join(tmp, "claude-code", "coding-style")
	canon := canonical.Normalize(body)
	wantHash := HashBody(body)

	// rule.md
	ruleMD, err := os.ReadFile(filepath.Join(dir, "rule.md"))
	if err != nil {
		t.Fatalf("reading rule.md: %v", err)
	}
	if !bytes.Equal(ruleMD, canon) {
		t.Errorf("rule.md: got %q, want %q", ruleMD, canon)
	}

	// .history/sha256-<64hex>.md
	historyName := hashToFilename(wantHash)
	historyBytes, err := os.ReadFile(filepath.Join(dir, ".history", historyName))
	if err != nil {
		t.Fatalf("reading history file %s: %v", historyName, err)
	}
	if !bytes.Equal(historyBytes, canon) {
		t.Errorf(".history body: got %q, want %q", historyBytes, canon)
	}

	// .syllago.yaml
	loaded, err := metadata.LoadRuleMetadata(filepath.Join(dir, metadata.FileName))
	if err != nil {
		t.Fatalf("LoadRuleMetadata: %v", err)
	}
	if len(loaded.Versions) != 1 {
		t.Fatalf("versions: got %d, want 1", len(loaded.Versions))
	}
	if loaded.Versions[0].Hash != wantHash {
		t.Errorf("versions[0].Hash: got %q, want %q", loaded.Versions[0].Hash, wantHash)
	}
	if loaded.CurrentVersion != wantHash {
		t.Errorf("current_version: got %q, want %q", loaded.CurrentVersion, wantHash)
	}
	if loaded.FormatVersion != metadata.CurrentFormatVersion {
		t.Errorf("format_version: got %d, want %d", loaded.FormatVersion, metadata.CurrentFormatVersion)
	}
	if loaded.Type != "rule" {
		t.Errorf("type: got %q, want %q", loaded.Type, "rule")
	}
	if loaded.Name != "Coding style" {
		t.Errorf("name: got %q, want %q", loaded.Name, "Coding style")
	}
}
