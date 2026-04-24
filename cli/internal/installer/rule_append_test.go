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

func TestAppendToTarget_SequentialThreeRules(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "CLAUDE.md")

	if err := os.WriteFile(target, []byte("P\n"), 0644); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	for _, body := range [][]byte{
		[]byte("r1\n"),
		[]byte("r2\n"),
		[]byte("r3\n"),
	} {
		if err := AppendRuleToTarget(target, body); err != nil {
			t.Fatalf("append %q: %v", body, err)
		}
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := "P\n\nr1\n\nr2\n\nr3\n"
	if string(got) != want {
		t.Errorf("file bytes mismatch\n got %q\nwant %q", got, want)
	}
}

func TestAppendToTarget_NonEmptyMissingNewline(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "CLAUDE.md")

	if err := os.WriteFile(target, []byte("P"), 0644); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	if err := AppendRuleToTarget(target, []byte("rule body\n")); err != nil {
		t.Fatalf("AppendRuleToTarget: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := "P\n\nrule body\n"
	if string(got) != want {
		t.Errorf("file bytes mismatch\n got %q\nwant %q", got, want)
	}
}

func TestAppendToTarget_NonEmptyEndsWithNewline(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "CLAUDE.md")

	if err := os.WriteFile(target, []byte("P\n"), 0644); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	if err := AppendRuleToTarget(target, []byte("rule body\n")); err != nil {
		t.Fatalf("AppendRuleToTarget: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := "P\n\nrule body\n"
	if string(got) != want {
		t.Errorf("file bytes mismatch\n got %q\nwant %q", got, want)
	}
}
