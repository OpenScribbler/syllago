package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateSymlink_Basic(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	source := filepath.Join(tmp, "source")
	os.WriteFile(source, []byte("content"), 0644)
	target := filepath.Join(tmp, "target")

	if err := CreateSymlink(source, target); err != nil {
		t.Fatalf("CreateSymlink: %v", err)
	}

	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}
}

func TestCreateSymlink_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	source := filepath.Join(tmp, "source")
	os.WriteFile(source, []byte("content"), 0644)
	target := filepath.Join(tmp, "deep", "nested", "target")

	if err := CreateSymlink(source, target); err != nil {
		t.Fatalf("CreateSymlink: %v", err)
	}

	if _, err := os.Lstat(target); err != nil {
		t.Fatalf("target not created: %v", err)
	}
}

func TestCreateSymlink_ReplacesExistingSymlink(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	source1 := filepath.Join(tmp, "source1")
	source2 := filepath.Join(tmp, "source2")
	os.WriteFile(source1, []byte("one"), 0644)
	os.WriteFile(source2, []byte("two"), 0644)

	target := filepath.Join(tmp, "target")

	// Create first symlink
	if err := CreateSymlink(source1, target); err != nil {
		t.Fatalf("CreateSymlink (first): %v", err)
	}

	// Replace with second symlink
	if err := CreateSymlink(source2, target); err != nil {
		t.Fatalf("CreateSymlink (replace): %v", err)
	}

	// Verify it points to source2
	link, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if link != source2 {
		t.Errorf("expected symlink to %s, got %s", source2, link)
	}
}

func TestCreateSymlink_ReplacesExistingFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	source := filepath.Join(tmp, "source")
	os.WriteFile(source, []byte("content"), 0644)

	target := filepath.Join(tmp, "target")
	os.WriteFile(target, []byte("old content"), 0644)

	if err := CreateSymlink(source, target); err != nil {
		t.Fatalf("CreateSymlink: %v", err)
	}

	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink after replacing regular file")
	}
}

func TestIsSymlinkedTo_True(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	source := filepath.Join(repoRoot, "content", "file")
	os.MkdirAll(filepath.Dir(source), 0755)
	os.WriteFile(source, []byte("content"), 0644)

	link := filepath.Join(tmp, "link")
	os.Symlink(source, link)

	if !IsSymlinkedTo(link, repoRoot) {
		t.Error("expected IsSymlinkedTo to return true")
	}
}

func TestIsSymlinkedTo_FalseWrongRoot(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	source := filepath.Join(tmp, "other", "file")
	os.MkdirAll(filepath.Dir(source), 0755)
	os.WriteFile(source, []byte("content"), 0644)

	link := filepath.Join(tmp, "link")
	os.Symlink(source, link)

	if IsSymlinkedTo(link, filepath.Join(tmp, "repo")) {
		t.Error("expected IsSymlinkedTo to return false for wrong root")
	}
}

func TestIsSymlinkedTo_NotASymlink(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	file := filepath.Join(tmp, "file")
	os.WriteFile(file, []byte("content"), 0644)

	if IsSymlinkedTo(file, tmp) {
		t.Error("regular file should not be treated as symlinked")
	}
}

func TestIsWindowsMount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		{"/mnt/c/Users/foo", true},
		{"/mnt/d/some/path", true},
		{"/home/user/.claude", false},
		{"/mnt", false},
		{"/mnt/", false},
		{"", false},
		{"/mnt/c", false},      // no trailing slash after drive letter
		{"/mnt/cc/foo", false}, // two-char mount name — not a Windows drive
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := IsWindowsMount(tt.path)
			if got != tt.want {
				t.Errorf("IsWindowsMount(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
