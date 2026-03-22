package add

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestTraceSymlinkTaint(t *testing.T) {
	t.Parallel()

	t.Run("symlink into library propagates taint", func(t *testing.T) {
		t.Parallel()
		// Setup: library item with private taint
		globalDir := t.TempDir()
		itemDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
		os.MkdirAll(itemDir, 0755)
		os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# Rule"), 0644)
		meta := &metadata.Meta{
			ID:               "test-id",
			Name:             "my-rule",
			SourceRegistry:   "acme/internal",
			SourceVisibility: "private",
		}
		metadata.Save(itemDir, meta)

		// Simulate provider install via symlink
		providerDir := t.TempDir()
		symlinkPath := filepath.Join(providerDir, "my-rule")
		os.Symlink(itemDir, symlinkPath)

		// Trace the symlink
		reg, vis := traceSymlinkTaint(filepath.Join(symlinkPath, "rule.md"), globalDir)
		if reg != "acme/internal" {
			t.Errorf("registry = %q, want %q", reg, "acme/internal")
		}
		if vis != "private" {
			t.Errorf("visibility = %q, want %q", vis, "private")
		}
	})

	t.Run("non-symlink returns empty", func(t *testing.T) {
		t.Parallel()
		globalDir := t.TempDir()
		dir := t.TempDir()
		f := filepath.Join(dir, "rule.md")
		os.WriteFile(f, []byte("# Rule"), 0644)

		reg, vis := traceSymlinkTaint(f, globalDir)
		if reg != "" || vis != "" {
			t.Errorf("non-symlink: got (%q, %q), want empty", reg, vis)
		}
	})

	t.Run("symlink outside library returns empty", func(t *testing.T) {
		t.Parallel()
		globalDir := t.TempDir()
		otherDir := t.TempDir()
		os.WriteFile(filepath.Join(otherDir, "rule.md"), []byte("# Rule"), 0644)

		providerDir := t.TempDir()
		symlinkPath := filepath.Join(providerDir, "my-rule")
		os.Symlink(otherDir, symlinkPath)

		reg, vis := traceSymlinkTaint(filepath.Join(symlinkPath, "rule.md"), globalDir)
		if reg != "" || vis != "" {
			t.Errorf("outside library: got (%q, %q), want empty", reg, vis)
		}
	})

	t.Run("empty globalDir returns empty", func(t *testing.T) {
		t.Parallel()
		reg, vis := traceSymlinkTaint("/some/path", "")
		if reg != "" || vis != "" {
			t.Errorf("empty globalDir: got (%q, %q), want empty", reg, vis)
		}
	})
}

func TestHashMatchTaint(t *testing.T) {
	t.Parallel()

	t.Run("matching hash propagates taint", func(t *testing.T) {
		t.Parallel()
		globalDir := t.TempDir()
		itemDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
		os.MkdirAll(itemDir, 0755)
		os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# Rule content"), 0644)

		content := []byte("# Rule content")
		hash := sourceHash(content)

		meta := &metadata.Meta{
			ID:               "test-id",
			Name:             "my-rule",
			SourceHash:       hash,
			SourceRegistry:   "acme/secret",
			SourceVisibility: "private",
		}
		metadata.Save(itemDir, meta)

		reg, vis := hashMatchTaint(hash, globalDir)
		if reg != "acme/secret" {
			t.Errorf("registry = %q, want %q", reg, "acme/secret")
		}
		if vis != "private" {
			t.Errorf("visibility = %q, want %q", vis, "private")
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		t.Parallel()
		globalDir := t.TempDir()
		itemDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
		os.MkdirAll(itemDir, 0755)
		os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("other"), 0644)

		meta := &metadata.Meta{
			ID:               "test-id",
			Name:             "my-rule",
			SourceHash:       sourceHash([]byte("other")),
			SourceRegistry:   "acme/secret",
			SourceVisibility: "private",
		}
		metadata.Save(itemDir, meta)

		reg, vis := hashMatchTaint(sourceHash([]byte("different")), globalDir)
		if reg != "" || vis != "" {
			t.Errorf("no match: got (%q, %q), want empty", reg, vis)
		}
	})

	t.Run("public items not matched", func(t *testing.T) {
		t.Parallel()
		globalDir := t.TempDir()
		itemDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
		os.MkdirAll(itemDir, 0755)
		os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# Public rule"), 0644)

		content := []byte("# Public rule")
		hash := sourceHash(content)

		meta := &metadata.Meta{
			ID:               "test-id",
			Name:             "my-rule",
			SourceHash:       hash,
			SourceRegistry:   "community/rules",
			SourceVisibility: "public",
		}
		metadata.Save(itemDir, meta)

		reg, vis := hashMatchTaint(hash, globalDir)
		if reg != "" || vis != "" {
			t.Errorf("public item: got (%q, %q), want empty", reg, vis)
		}
	})

	t.Run("empty globalDir returns empty", func(t *testing.T) {
		t.Parallel()
		reg, vis := hashMatchTaint("sha256:abc", "")
		if reg != "" || vis != "" {
			t.Errorf("empty globalDir: got (%q, %q), want empty", reg, vis)
		}
	})
}
