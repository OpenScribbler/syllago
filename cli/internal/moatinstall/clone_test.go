package moatinstall

// Tests for the source-repo clone helpers (bead syllago-cvwj5). The real
// `git clone` is exercised end-to-end in install_moat_integration_test.go
// via the CloneRepoFn seam; here we cover the input-validation and
// directory-copy helpers that surround it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSourceURI(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		uri       string
		wantErr   bool
		wantInErr string
	}{
		{"https github", "https://github.com/owner/repo", false, ""},
		{"https deep path", "https://github.com/owner/repo.git", false, ""},
		{"empty", "", true, "empty"},
		{"git scheme", "git://github.com/owner/repo", true, "scheme not supported"},
		{"git+https scheme", "git+https://github.com/owner/repo", true, "scheme not supported"},
		{"http scheme", "http://github.com/owner/repo", true, "scheme not supported"},
		{"ssh scheme", "ssh://git@github.com/owner/repo", true, "scheme not supported"},
		{"missing host", "https:///owner/repo", true, "missing host or path"},
		{"missing path", "https://github.com", true, "missing host or path"},
		{"root path only", "https://github.com/", true, "missing host or path"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateSourceURI(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.uri)
				}
				if tc.wantInErr != "" && !strings.Contains(err.Error(), tc.wantInErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantInErr)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tc.uri, err)
			}
		})
	}
}

func TestCopyTree_HappyPath(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "skills", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "skills", "x", "SKILL.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "dst")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "skills", "x", "SKILL.md"))
	if err != nil || string(got) != "# x\n" {
		t.Errorf("nested file did not round-trip: got %q err=%v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(dst, "README.md"))
	if err != nil || string(got) != "readme\n" {
		t.Errorf("top-level file did not round-trip: got %q err=%v", got, err)
	}
}

func TestCopyTree_RejectsSymlinks(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "real.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.txt", filepath.Join(src, "link.txt")); err != nil {
		t.Skipf("cannot create symlink on this filesystem: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "dst")
	err := copyTree(src, dst)
	if err == nil || !strings.Contains(err.Error(), "symlink rejected") {
		t.Errorf("expected symlink-rejected error; got %v", err)
	}
}

func TestCopyTree_OverwritesExistingDst(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "stale.txt")); !os.IsNotExist(err) {
		t.Errorf("stale.txt should have been removed; got err=%v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "new.txt")); err != nil || string(got) != "new" {
		t.Errorf("new.txt missing or wrong: got %q err=%v", got, err)
	}
}

func TestCheckGit(t *testing.T) {
	t.Parallel()
	// On dev machines git is on PATH. We don't validate the negative case
	// because mucking with PATH is not test-isolated.
	if err := checkGit(); err != nil {
		t.Skipf("git not on PATH; skipping: %v", err)
	}
}
