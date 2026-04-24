package add

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectSplittable_UnrecognizedFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte("# hi\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	splittable, n, err := DetectSplittable(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if splittable {
		t.Fatalf("README.md must not be splittable")
	}
	if n != 0 {
		t.Fatalf("expected section count 0, got %d", n)
	}
}

func TestDetectSplittable_MissingFile(t *testing.T) {
	t.Parallel()
	_, _, err := DetectSplittable(filepath.Join(t.TempDir(), "CLAUDE.md"))
	if err == nil {
		t.Fatal("expected read error for missing file")
	}
}

func TestDetectSplittable_TooSmall(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(path, []byte("# Short\n## A\nbody\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	splittable, n, err := DetectSplittable(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if splittable {
		t.Fatalf("short CLAUDE.md must not be splittable")
	}
	if n != 0 {
		t.Fatalf("expected section count 0, got %d", n)
	}
}

func TestDetectSplittable_H2ValidFile(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("# Project\n")
	for i := 0; i < 3; i++ {
		b.WriteString("## Section ")
		b.WriteByte(byte('A' + i))
		b.WriteString("\n")
		for j := 0; j < 12; j++ {
			b.WriteString("line of body content\n")
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	splittable, n, err := DetectSplittable(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !splittable {
		t.Fatalf("valid CLAUDE.md must be splittable")
	}
	if n != 3 {
		t.Fatalf("expected 3 sections, got %d", n)
	}
}

func TestDetectSplittable_RecognizesAgentsMD(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	for i := 0; i < 3; i++ {
		b.WriteString("## Rule ")
		b.WriteByte(byte('X' + i))
		b.WriteString("\n")
		for j := 0; j < 12; j++ {
			b.WriteString("more body text here\n")
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	splittable, n, err := DetectSplittable(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !splittable {
		t.Fatalf("AGENTS.md must be recognized as monolithic")
	}
	if n != 3 {
		t.Fatalf("expected 3 sections, got %d", n)
	}
}
