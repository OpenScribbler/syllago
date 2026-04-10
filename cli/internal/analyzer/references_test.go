package analyzer

import (
	"path/filepath"
	"testing"
)

func TestResolveReferences_MarkdownLink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "skills/my-skill/SKILL.md", "See [helper](./helpers/foo.sh) for details.\n")
	setupFile(t, root, "skills/my-skill/helpers/foo.sh", "#!/bin/sh\n")

	refs := ResolveReferences("skills/my-skill/SKILL.md", root)
	found := false
	for _, r := range refs {
		if r == filepath.Join("skills", "my-skill", "helpers", "foo.sh") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected helpers/foo.sh in refs, got %v", refs)
	}
}

func TestResolveReferences_NonExistentLink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "skills/my-skill/SKILL.md", "See [ghost](./ghost.md) for details.\n")

	refs := ResolveReferences("skills/my-skill/SKILL.md", root)
	for _, r := range refs {
		if r == filepath.Join("skills", "my-skill", "ghost.md") {
			t.Error("non-existent file should not be in refs")
		}
	}
}

func TestResolveReferences_BacktickPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "agents/reviewer/AGENT.md", "Uses `scripts/lint.ts` for linting.\n")
	setupFile(t, root, "agents/reviewer/scripts/lint.ts", "// lint\n")

	refs := ResolveReferences("agents/reviewer/AGENT.md", root)
	found := false
	for _, r := range refs {
		if r == filepath.Join("agents", "reviewer", "scripts", "lint.ts") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected scripts/lint.ts in refs, got %v", refs)
	}
}

func TestResolveReferences_UnknownExtension(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "skills/my-skill/SKILL.md", "See `image.png` for the logo.\n")
	setupFile(t, root, "skills/my-skill/image.png", "fake png")

	refs := ResolveReferences("skills/my-skill/SKILL.md", root)
	for _, r := range refs {
		if filepath.Ext(r) == ".png" {
			t.Error(".png should not be resolved as a content reference")
		}
	}
}

func TestResolveReferences_PathTraversal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "skills/evil/SKILL.md", "See [secret](../../etc/passwd) for details.\n")

	refs := ResolveReferences("skills/evil/SKILL.md", root)
	for _, r := range refs {
		if r == "etc/passwd" || r == "../etc/passwd" {
			t.Error("path traversal should be blocked")
		}
	}
}

func TestResolveReferences_KnownSubdir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "skills/my-skill/SKILL.md", "# My Skill\n")
	setupFile(t, root, "skills/my-skill/references/guide.md", "# Guide\n")
	setupFile(t, root, "skills/my-skill/references/faq.md", "# FAQ\n")

	refs := ResolveReferences("skills/my-skill/SKILL.md", root)
	if len(refs) < 2 {
		t.Errorf("expected at least 2 refs from references/ subdir, got %d: %v", len(refs), refs)
	}
}

func TestResolveReferences_NoDuplicates(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Reference the same file via markdown link and backtick.
	setupFile(t, root, "skills/my-skill/SKILL.md", "See [helper](./helper.sh) and `helper.sh`.\n")
	setupFile(t, root, "skills/my-skill/helper.sh", "#!/bin/sh\n")

	refs := ResolveReferences("skills/my-skill/SKILL.md", root)
	seen := make(map[string]int)
	for _, r := range refs {
		seen[r]++
	}
	for path, count := range seen {
		if count > 1 {
			t.Errorf("duplicate ref: %q appears %d times", path, count)
		}
	}
}
