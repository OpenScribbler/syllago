package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyContent(t *testing.T) {
	t.Parallel()
	t.Run("copy single file", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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

	t.Run("copyFile refuses symlink at destination", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()

		// Create a source file
		srcFile := filepath.Join(tmp, "source.txt")
		if err := os.WriteFile(srcFile, []byte("attack payload"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a target file that we don't want overwritten
		targetFile := filepath.Join(tmp, "important.txt")
		if err := os.WriteFile(targetFile, []byte("important data"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a symlink at the destination pointing to the important file
		symlinkPath := filepath.Join(tmp, "dest.txt")
		if err := os.Symlink(targetFile, symlinkPath); err != nil {
			t.Fatal(err)
		}

		// Attempt to copy — should fail
		err := copyFile(srcFile, symlinkPath)
		if err == nil {
			t.Fatal("copyFile should refuse to follow symlink at destination")
		}

		if !strings.Contains(err.Error(), "symlink") {
			t.Errorf("error should mention symlink, got: %v", err)
		}

		// Verify important file was not overwritten
		data, err := os.ReadFile(targetFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "important data" {
			t.Errorf("target file was overwritten! got: %s", data)
		}
	})

	t.Run("copyFile works for normal files", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()

		srcFile := filepath.Join(tmp, "source.txt")
		if err := os.WriteFile(srcFile, []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}

		dstFile := filepath.Join(tmp, "dest.txt")
		if err := copyFile(srcFile, dstFile); err != nil {
			t.Fatalf("copyFile failed for normal file: %v", err)
		}

		data, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "test content" {
			t.Errorf("content mismatch: got %s", data)
		}
	})

	t.Run("copyDir skips symlinks in source tree", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()

		srcDir := filepath.Join(tmp, "src")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create a normal file
		if err := os.WriteFile(filepath.Join(srcDir, "normal.txt"), []byte("normal content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a symlink to a sensitive file (simulated)
		sensitiveFile := filepath.Join(tmp, "sensitive.txt")
		if err := os.WriteFile(sensitiveFile, []byte("SECRET DATA"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(sensitiveFile, filepath.Join(srcDir, "sneaky.txt")); err != nil {
			t.Fatal(err)
		}

		dstDir := filepath.Join(tmp, "dst")
		if err := copyDir(srcDir, dstDir); err != nil {
			t.Fatalf("copyDir failed: %v", err)
		}

		// Verify normal file was copied
		if _, err := os.Stat(filepath.Join(dstDir, "normal.txt")); err != nil {
			t.Errorf("normal file was not copied: %v", err)
		}

		// Verify symlink was NOT copied
		if _, err := os.Stat(filepath.Join(dstDir, "sneaky.txt")); err == nil {
			data, _ := os.ReadFile(filepath.Join(dstDir, "sneaky.txt"))
			t.Errorf("symlink should not have been copied, got: %s", data)
		}
	})

	t.Run("source does not exist returns error", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		src := filepath.Join(tmp, "nonexistent")
		dst := filepath.Join(tmp, "dst")

		err := CopyContent(src, dst)
		if err == nil {
			t.Fatal("expected error for nonexistent source, got nil")
		}
	})

	t.Run("copied file contents match source", func(t *testing.T) {
		t.Parallel()
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
