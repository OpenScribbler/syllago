package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppendToTarget_EmptyFileCreates(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "CLAUDE.md")

	body := []byte("# Rule\n\nThis is the body.\n")
	if err := AppendRuleToTarget(target, body); err != nil {
		t.Fatalf("AppendRuleToTarget: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := append([]byte{'\n'}, body...)
	if string(got) != string(want) {
		t.Errorf("file bytes mismatch\n got %q\nwant %q", got, want)
	}
}
