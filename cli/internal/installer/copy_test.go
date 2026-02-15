package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyContent(t *testing.T) {
	t.Run("copy single file", func(t *testing.T) {
		tmp := t.TempDir()
		srcFile := filepath.Join(tmp, "src", "hello.txt")
		dstFile := filepath.Join(tmp, "dst", "hello.txt")

		if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(srcFile, []byte("hello world"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := CopyContent(srcFile, dstFile); err != nil {
			t.Fatalf("CopyContent returned error: %v", err)
		}

		got, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatalf("failed to read copied file: %v", err)
		}
		if string(got) != "hello world" {
			t.Errorf("copied content = %q, want %q", string(got), "hello world")
		}
	})

	t.Run("copy directory with subdirectories", func(t *testing.T) {
		tmp := t.TempDir()
		srcDir := filepath.Join(tmp, "src")
		dstDir := filepath.Join(tmp, "dst")

		// Create source structure: src/a.txt, src/sub/b.txt
		if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("file a"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("file b"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := CopyContent(srcDir, dstDir); err != nil {
			t.Fatalf("CopyContent returned error: %v", err)
		}

		// Verify a.txt
		gotA, err := os.ReadFile(filepath.Join(dstDir, "a.txt"))
		if err != nil {
			t.Fatalf("failed to read dst/a.txt: %v", err)
		}
		if string(gotA) != "file a" {
			t.Errorf("a.txt content = %q, want %q", string(gotA), "file a")
		}

		// Verify sub/b.txt
		gotB, err := os.ReadFile(filepath.Join(dstDir, "sub", "b.txt"))
		if err != nil {
			t.Fatalf("failed to read dst/sub/b.txt: %v", err)
		}
		if string(gotB) != "file b" {
			t.Errorf("sub/b.txt content = %q, want %q", string(gotB), "file b")
		}
	})

	t.Run("source does not exist returns error", func(t *testing.T) {
		tmp := t.TempDir()
		src := filepath.Join(tmp, "nonexistent")
		dst := filepath.Join(tmp, "dst")

		err := CopyContent(src, dst)
		if err == nil {
			t.Fatal("expected error for nonexistent source, got nil")
		}
	})

	t.Run("copied file contents match source", func(t *testing.T) {
		tmp := t.TempDir()

		// Use binary-ish content to make sure nothing is mangled.
		content := "line1\nline2\ttab\x00null byte"
		srcFile := filepath.Join(tmp, "src.bin")
		dstFile := filepath.Join(tmp, "dst.bin")

		if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		if err := CopyContent(srcFile, dstFile); err != nil {
			t.Fatalf("CopyContent returned error: %v", err)
		}

		got, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatalf("failed to read copied file: %v", err)
		}
		if string(got) != content {
			t.Errorf("copied content does not match source.\ngot:  %q\nwant: %q", string(got), content)
		}
	})
}
