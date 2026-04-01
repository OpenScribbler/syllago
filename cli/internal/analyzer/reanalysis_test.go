package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestDiffManifest_Unchanged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "unchanged content"
	setupFile(t, root, "skills/my-skill/SKILL.md", content)

	m := &registry.Manifest{
		Items: []registry.ManifestItem{
			{Name: "my-skill", Path: "skills/my-skill/SKILL.md", ContentHash: sha256hex(content)},
		},
	}

	result := DiffManifest(m, root)
	if len(result.Unchanged) != 1 {
		t.Errorf("Unchanged = %d, want 1", len(result.Unchanged))
	}
	if len(result.Changed) != 0 {
		t.Errorf("Changed = %d, want 0", len(result.Changed))
	}
}

func TestDiffManifest_Changed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "skills/my-skill/SKILL.md", "new content")

	m := &registry.Manifest{
		Items: []registry.ManifestItem{
			{Name: "my-skill", Path: "skills/my-skill/SKILL.md", ContentHash: sha256hex("old content")},
		},
	}

	result := DiffManifest(m, root)
	if len(result.Changed) != 1 {
		t.Errorf("Changed = %d, want 1", len(result.Changed))
	}
}

func TestDiffManifest_Missing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	m := &registry.Manifest{
		Items: []registry.ManifestItem{
			{Name: "gone", Path: "skills/gone/SKILL.md", ContentHash: "abc"},
		},
	}

	result := DiffManifest(m, root)
	if len(result.Missing) != 1 {
		t.Errorf("Missing = %d, want 1", len(result.Missing))
	}
}

func TestDiffManifest_EmptyHash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	m := &registry.Manifest{
		Items: []registry.ManifestItem{
			{Name: "authored", Path: "skills/authored/SKILL.md", ContentHash: ""},
		},
	}

	result := DiffManifest(m, root)
	if len(result.Unchanged) != 1 {
		t.Errorf("Unchanged = %d, want 1 (empty hash = authored, always unchanged)", len(result.Unchanged))
	}
}

func TestPreserveUserMetadata_PreservesEmpty(t *testing.T) {
	t.Parallel()

	existing := &registry.ManifestItem{DisplayName: "User Name", Description: "User Desc"}
	detected := &DetectedItem{}

	PreserveUserMetadata(existing, detected)
	if detected.DisplayName != "User Name" {
		t.Errorf("DisplayName = %q, want %q", detected.DisplayName, "User Name")
	}
	if detected.Description != "User Desc" {
		t.Errorf("Description = %q, want %q", detected.Description, "User Desc")
	}
}

func TestPreserveUserMetadata_DoesNotOverwrite(t *testing.T) {
	t.Parallel()

	existing := &registry.ManifestItem{DisplayName: "User Name"}
	detected := &DetectedItem{DisplayName: "Detected Name"}

	PreserveUserMetadata(existing, detected)
	if detected.DisplayName != "Detected Name" {
		t.Errorf("DisplayName = %q, want %q (should not be overwritten)", detected.DisplayName, "Detected Name")
	}
}
