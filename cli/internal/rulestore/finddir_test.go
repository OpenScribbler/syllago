package rulestore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestFindRuleDir(t *testing.T) {
	root := t.TempDir()
	ruleDir := filepath.Join(root, "claude-code", "concise-comments")
	if err := os.MkdirAll(ruleDir, 0755); err != nil {
		t.Fatalf("MkdirAll ruleDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "not-a-provider"), []byte("skip\n"), 0644); err != nil {
		t.Fatalf("WriteFile non-dir: %v", err)
	}

	got, err := FindRuleDir(root, "concise-comments")
	if err != nil {
		t.Fatalf("FindRuleDir: %v", err)
	}
	if got != ruleDir {
		t.Fatalf("FindRuleDir = %s, want %s", got, ruleDir)
	}

	_, err = FindRuleDir(root, "missing")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("FindRuleDir missing err = %v, want fs.ErrNotExist", err)
	}
}
