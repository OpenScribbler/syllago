package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// TestAddSplitRuleItem_WritesSections verifies that addSplitRuleItem runs the
// splitter on a monolithic CLAUDE.md, writes each section under the provider
// slug, and tags the per-section metadata with the source block.
func TestAddSplitRuleItem_WritesSections(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	claudePath := writeSplittableClaudeMD(t, tmp)
	contentRoot := t.TempDir()

	item := addDiscoveryItem{
		name:              "CLAUDE",
		itemType:          catalog.Rules,
		status:            add.StatusNew,
		path:              claudePath,
		splittable:        true,
		splitSectionCount: 3,
		splitChosen:       true,
	}

	res := addSplitRuleItem(item, contentRoot, "claude-code")
	if res.status != "added" {
		t.Fatalf("expected status=added, got %q (err=%v)", res.status, res.err)
	}
	if !strings.Contains(res.name, "3 sections") {
		t.Fatalf("expected section count in result name, got %q", res.name)
	}

	// claude-code/<slug>/ directories should exist for each section.
	providerDir := filepath.Join(contentRoot, "claude-code")
	entries, err := os.ReadDir(providerDir)
	if err != nil {
		t.Fatalf("reading %s: %v", providerDir, err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 rule dirs under claude-code/, got %d", len(entries))
	}

	// Each rule should have a .syllago.yaml with a populated source block.
	for _, e := range entries {
		metaPath := filepath.Join(providerDir, e.Name(), metadata.FileName)
		meta, merr := metadata.LoadRuleMetadata(metaPath)
		if merr != nil {
			t.Fatalf("loading %s: %v", metaPath, merr)
		}
		if meta.Source.Filename != "CLAUDE.md" {
			t.Errorf("%s: expected source.filename=CLAUDE.md, got %q", e.Name(), meta.Source.Filename)
		}
		if meta.Source.Format != "claude-code" {
			t.Errorf("%s: expected source.format=claude-code, got %q", e.Name(), meta.Source.Format)
		}
		if meta.Source.SplitMethod != "h2" {
			t.Errorf("%s: expected source.split_method=h2, got %q", e.Name(), meta.Source.SplitMethod)
		}
		if meta.Source.Hash == "" {
			t.Errorf("%s: expected non-empty source.hash", e.Name())
		}
	}

	// The original source file should be preserved under .source/CLAUDE.md.
	for _, e := range entries {
		srcCopy := filepath.Join(providerDir, e.Name(), ".source", "CLAUDE.md")
		if _, err := os.Stat(srcCopy); err != nil {
			t.Errorf("expected %s to exist: %v", srcCopy, err)
		}
	}
}

func TestAddSplitRuleItem_EmptyPath(t *testing.T) {
	t.Parallel()
	item := addDiscoveryItem{
		name:        "CLAUDE",
		itemType:    catalog.Rules,
		splittable:  true,
		splitChosen: true,
	}
	res := addSplitRuleItem(item, t.TempDir(), "claude-code")
	if res.status != "error" {
		t.Fatalf("expected status=error for empty path, got %q", res.status)
	}
}

func TestAddSplitRuleItem_LocalFallbackSlug(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	claudePath := writeSplittableClaudeMD(t, tmp)
	contentRoot := t.TempDir()

	item := addDiscoveryItem{
		name:              "CLAUDE",
		itemType:          catalog.Rules,
		status:            add.StatusNew,
		path:              claudePath,
		splittable:        true,
		splitSectionCount: 3,
		splitChosen:       true,
	}

	// Empty provSlug → should fall back to "local" directory.
	res := addSplitRuleItem(item, contentRoot, "")
	if res.status != "added" {
		t.Fatalf("expected status=added, got %q (err=%v)", res.status, res.err)
	}
	if _, err := os.Stat(filepath.Join(contentRoot, "local")); err != nil {
		t.Fatalf("expected local/ dir when provSlug empty: %v", err)
	}
}
