package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeResolve(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "file.md"), []byte("x"), 0644)

	outer := t.TempDir()
	os.WriteFile(filepath.Join(outer, "secret.txt"), []byte("s"), 0644)

	tests := []struct {
		name      string
		base      string
		untrusted string
		wantErr   bool
	}{
		{"safe nested", dir, "sub/file.md", false},
		{"safe direct", dir, "file.md", false},
		{"traversal dotdot", dir, "../secret.txt", true},
		{"traversal absolute", dir, outer, true},
		{"traversal embedded", dir, "sub/../../secret.txt", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SafeResolve(tt.base, tt.untrusted)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q", tt.untrusted)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSafeResolve_SymlinkEscape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outer := t.TempDir()
	os.WriteFile(filepath.Join(outer, "secret.txt"), []byte("s"), 0644)
	// Create a symlink inside dir that points outside
	os.Symlink(outer, filepath.Join(dir, "escape"))

	_, err := SafeResolve(dir, "escape/secret.txt")
	if err == nil {
		t.Error("expected error for symlink escape, got nil")
	}
}
