package add

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- contentFilename (40% coverage) ---

func TestContentFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ct   catalog.ContentType
		item string
		ext  string
		want string
	}{
		{"rules .md", catalog.Rules, "my-rule", ".md", "rule.md"},
		{"rules .txt", catalog.Rules, "my-rule", ".txt", "rule.txt"},
		{"rules empty ext", catalog.Rules, "my-rule", "", "rule.md"},
		{"hooks", catalog.Hooks, "my-hook", ".json", "hook.json"},
		{"hooks ignores ext", catalog.Hooks, "my-hook", ".md", "hook.json"},
		{"commands .md", catalog.Commands, "my-cmd", ".md", "command.md"},
		{"commands .txt", catalog.Commands, "my-cmd", ".txt", "command.txt"},
		{"skills", catalog.Skills, "my-skill", ".md", "SKILL.md"},
		{"skills ignores ext", catalog.Skills, "my-skill", ".txt", "SKILL.md"},
		{"agents", catalog.Agents, "my-agent", ".md", "agent.md"},
		{"agents ignores ext", catalog.Agents, "my-agent", ".txt", "agent.md"},
		{"mcp", catalog.MCP, "my-mcp", ".json", "mcp.json"},
		{"mcp ignores ext", catalog.MCP, "my-mcp", ".md", "mcp.json"},
		{"default type", catalog.Loadouts, "my-loadout", ".yaml", "my-loadout.yaml"},
		{"default empty ext", catalog.Loadouts, "my-loadout", "", "my-loadout.md"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := contentFilename(tc.ct, tc.item, tc.ext)
			if got != tc.want {
				t.Errorf("contentFilename(%q, %q, %q) = %q, want %q", tc.ct, tc.item, tc.ext, got, tc.want)
			}
		})
	}
}

// --- computeItemStatus (0% coverage) ---

func TestComputeItemStatus_New(t *testing.T) {
	t.Parallel()
	// Item not in library → StatusNew
	idx := make(LibraryIndex)
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "rule.md")
	os.WriteFile(filePath, []byte("# Rule"), 0644)

	status := computeItemStatus(filePath, catalog.Rules, "claude-code", "my-rule", idx)
	if status != StatusNew {
		t.Errorf("computeItemStatus() = %v, want StatusNew", status)
	}
}

func TestComputeItemStatus_InLibrary(t *testing.T) {
	t.Parallel()
	// Item in library with matching hash → StatusInLibrary
	tmp := t.TempDir()
	content := []byte("# My Rule")
	filePath := filepath.Join(tmp, "rule.md")
	os.WriteFile(filePath, content, 0644)

	hash := sourceHash(content)
	idx := LibraryIndex{
		"rules/claude-code/my-rule": &metadata.Meta{SourceHash: hash},
	}

	status := computeItemStatus(filePath, catalog.Rules, "claude-code", "my-rule", idx)
	if status != StatusInLibrary {
		t.Errorf("computeItemStatus() = %v, want StatusInLibrary", status)
	}
}

func TestComputeItemStatus_Outdated(t *testing.T) {
	t.Parallel()
	// Item in library with different hash → StatusOutdated
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "rule.md")
	os.WriteFile(filePath, []byte("# Updated Rule"), 0644)

	idx := LibraryIndex{
		"rules/claude-code/my-rule": &metadata.Meta{SourceHash: "sha256:oldoldhash"},
	}

	status := computeItemStatus(filePath, catalog.Rules, "claude-code", "my-rule", idx)
	if status != StatusOutdated {
		t.Errorf("computeItemStatus() = %v, want StatusOutdated", status)
	}
}

func TestComputeItemStatus_InLibrary_NilMeta(t *testing.T) {
	t.Parallel()
	// Item exists in library but has no metadata (nil value) → StatusOutdated
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "rule.md")
	os.WriteFile(filePath, []byte("# Rule"), 0644)

	idx := LibraryIndex{
		"rules/claude-code/my-rule": nil, // dir exists, no .syllago.yaml
	}

	status := computeItemStatus(filePath, catalog.Rules, "claude-code", "my-rule", idx)
	if status != StatusOutdated {
		t.Errorf("computeItemStatus() = %v, want StatusOutdated (nil meta)", status)
	}
}

func TestComputeItemStatus_UnreadableFile(t *testing.T) {
	t.Parallel()
	// File can't be read → StatusNew (error surfaces on add)
	idx := LibraryIndex{
		"rules/claude-code/my-rule": &metadata.Meta{SourceHash: "sha256:abc"},
	}

	status := computeItemStatus("/nonexistent/file.md", catalog.Rules, "claude-code", "my-rule", idx)
	if status != StatusNew {
		t.Errorf("computeItemStatus() = %v, want StatusNew (unreadable file)", status)
	}
}

func TestComputeItemStatus_UniversalType(t *testing.T) {
	t.Parallel()
	// Universal type uses "type/name" key
	tmp := t.TempDir()
	content := []byte("# Skill")
	filePath := filepath.Join(tmp, "SKILL.md")
	os.WriteFile(filePath, content, 0644)

	hash := sourceHash(content)
	idx := LibraryIndex{
		"skills/my-skill": &metadata.Meta{SourceHash: hash},
	}

	status := computeItemStatus(filePath, catalog.Skills, "claude-code", "my-skill", idx)
	if status != StatusInLibrary {
		t.Errorf("computeItemStatus() = %v, want StatusInLibrary (universal)", status)
	}
}

// --- DiscoverFromProvider (0% coverage) ---

func TestDiscoverFromProvider_Basic(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	// Create a rules directory with one rule file
	rulesDir := filepath.Join(projectRoot, ".claude", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "my-rule.md"), []byte("# My Rule"), 0644)

	prov := provider.Provider{
		Slug: "claude-code",
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
		DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
			if ct == catalog.Rules {
				return []string{filepath.Join(projectRoot, ".claude", "rules")}
			}
			return nil
		},
	}

	items, err := DiscoverFromProvider(prov, projectRoot, nil, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromProvider: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Name != "my-rule" {
		t.Errorf("Name = %q, want %q", items[0].Name, "my-rule")
	}
	if items[0].Type != catalog.Rules {
		t.Errorf("Type = %q, want Rules", items[0].Type)
	}
	if items[0].Status != StatusNew {
		t.Errorf("Status = %v, want StatusNew", items[0].Status)
	}
}

func TestDiscoverFromProvider_SkipsHooksAndMCP(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	prov := provider.Provider{
		Slug: "claude-code",
		SupportsType: func(ct catalog.ContentType) bool {
			return true // supports everything
		},
		DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
			// Return something for all types — hooks/MCP should be filtered out
			return []string{projectRoot}
		},
	}

	items, err := DiscoverFromProvider(prov, projectRoot, nil, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromProvider: %v", err)
	}

	for _, item := range items {
		if item.Type == catalog.Hooks || item.Type == catalog.MCP {
			t.Errorf("unexpected item type %s — hooks/MCP should be skipped", item.Type)
		}
	}
}

func TestDiscoverFromProvider_NilDiscoveryPaths(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	prov := provider.Provider{
		Slug:           "claude-code",
		DiscoveryPaths: nil, // no discovery paths function
	}

	items, err := DiscoverFromProvider(prov, projectRoot, nil, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromProvider: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (no DiscoveryPaths)", len(items))
	}
}

func TestDiscoverFromProvider_DeduplicatesItems(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	// Create same-named file in two different discovery paths
	dir1 := filepath.Join(projectRoot, "path1")
	dir2 := filepath.Join(projectRoot, "path2")
	os.MkdirAll(dir1, 0755)
	os.MkdirAll(dir2, 0755)
	os.WriteFile(filepath.Join(dir1, "my-rule.md"), []byte("# Rule v1"), 0644)
	os.WriteFile(filepath.Join(dir2, "my-rule.md"), []byte("# Rule v2"), 0644)

	prov := provider.Provider{
		Slug: "claude-code",
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
		DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
			if ct == catalog.Rules {
				return []string{dir1, dir2}
			}
			return nil
		},
	}

	items, err := DiscoverFromProvider(prov, projectRoot, nil, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromProvider: %v", err)
	}

	// Should only have 1 item (deduplication by type/name)
	if len(items) != 1 {
		t.Errorf("got %d items, want 1 (deduplication)", len(items))
	}
}

func TestDiscoverFromProvider_WithExistingLibraryItem(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	content := []byte("# My Rule\nDo things.")

	// Create a rule in the provider
	rulesDir := filepath.Join(projectRoot, "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "my-rule.md"), content, 0644)

	// Create same rule in library with matching hash
	libRuleDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
	os.MkdirAll(libRuleDir, 0755)
	os.WriteFile(filepath.Join(libRuleDir, "rule.md"), content, 0644)
	meta := &metadata.Meta{
		ID:         "test-id",
		Name:       "my-rule",
		SourceHash: sourceHash(content),
	}
	metadata.Save(libRuleDir, meta)

	prov := provider.Provider{
		Slug: "claude-code",
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
		DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
			if ct == catalog.Rules {
				return []string{filepath.Join(projectRoot, "rules")}
			}
			return nil
		},
	}

	items, err := DiscoverFromProvider(prov, projectRoot, nil, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromProvider: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Status != StatusInLibrary {
		t.Errorf("Status = %v, want StatusInLibrary", items[0].Status)
	}
}

// --- writeItem edge cases ---

func TestWriteItem_Canonicalization(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(projectRoot, "my-rule.cursorrules")
	os.WriteFile(srcPath, []byte("original content"), 0644)

	canon := &mockCanonicalizer{
		content:  []byte("# Canonicalized\nrule content"),
		filename: "rule.md",
	}

	item := DiscoveryItem{
		Name:   "my-rule",
		Type:   catalog.Rules,
		Path:   srcPath,
		Status: StatusNew,
	}
	opts := AddOptions{Provider: "cursor"}

	result := writeItem(item, opts, globalDir, canon, "test-v1")
	if result.Status != AddStatusAdded {
		t.Fatalf("Status = %v, want AddStatusAdded", result.Status)
	}

	// Verify canonicalized content was written
	destFile := filepath.Join(globalDir, "rules", "cursor", "my-rule", "rule.md")
	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if string(data) != "# Canonicalized\nrule content" {
		t.Errorf("content = %q, want canonicalized content", string(data))
	}

	// Verify source was preserved in .source/
	sourcePath := filepath.Join(globalDir, "rules", "cursor", "my-rule", ".source", "my-rule.cursorrules")
	data, err = os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("reading .source: %v", err)
	}
	if string(data) != "original content" {
		t.Errorf(".source content = %q, want original", string(data))
	}
}

func TestWriteItem_CanonicalizeError(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	srcPath := filepath.Join(projectRoot, "my-rule.md")
	os.WriteFile(srcPath, []byte("# Fallback content"), 0644)

	canon := &mockCanonicalizer{err: os.ErrNotExist}

	item := DiscoveryItem{
		Name:   "my-rule",
		Type:   catalog.Rules,
		Path:   srcPath,
		Status: StatusNew,
	}
	opts := AddOptions{Provider: "claude-code"}

	result := writeItem(item, opts, globalDir, canon, "test-v1")
	if result.Status != AddStatusAdded {
		t.Fatalf("Status = %v, want AddStatusAdded (fallback to raw)", result.Status)
	}

	// Should have written the raw content (fallback)
	destFile := filepath.Join(globalDir, "rules", "claude-code", "my-rule", "rule.md")
	data, _ := os.ReadFile(destFile)
	if string(data) != "# Fallback content" {
		t.Errorf("content = %q, want raw fallback content", string(data))
	}
}

func TestWriteItem_EmptyVersion(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	srcPath := filepath.Join(projectRoot, "my-rule.md")
	os.WriteFile(srcPath, []byte("# Rule"), 0644)

	item := DiscoveryItem{
		Name:   "my-rule",
		Type:   catalog.Rules,
		Path:   srcPath,
		Status: StatusNew,
	}
	opts := AddOptions{Provider: "claude-code"}

	result := writeItem(item, opts, globalDir, nil, "")
	if result.Status != AddStatusAdded {
		t.Fatalf("Status = %v, want AddStatusAdded", result.Status)
	}

	// Check metadata has default version
	destDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
	meta, err := metadata.Load(destDir)
	if err != nil {
		t.Fatalf("metadata.Load: %v", err)
	}
	if meta.AddedBy != "syllago" {
		t.Errorf("AddedBy = %q, want %q (default)", meta.AddedBy, "syllago")
	}
}

func TestWriteItem_RegistryTaint(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	srcPath := filepath.Join(projectRoot, "my-rule.md")
	os.WriteFile(srcPath, []byte("# Rule"), 0644)

	item := DiscoveryItem{
		Name:   "my-rule",
		Type:   catalog.Rules,
		Path:   srcPath,
		Status: StatusNew,
	}
	opts := AddOptions{
		Provider:         "claude-code",
		SourceRegistry:   "acme/private-tools",
		SourceVisibility: "private",
	}

	result := writeItem(item, opts, globalDir, nil, "test-v1")
	if result.Status != AddStatusAdded {
		t.Fatalf("Status = %v, want AddStatusAdded", result.Status)
	}

	destDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
	meta, err := metadata.Load(destDir)
	if err != nil {
		t.Fatalf("metadata.Load: %v", err)
	}
	if meta.SourceRegistry != "acme/private-tools" {
		t.Errorf("SourceRegistry = %q, want %q", meta.SourceRegistry, "acme/private-tools")
	}
	if meta.SourceVisibility != "private" {
		t.Errorf("SourceVisibility = %q, want %q", meta.SourceVisibility, "private")
	}
	if meta.SourceType != "registry" {
		t.Errorf("SourceType = %q, want %q", meta.SourceType, "registry")
	}
}

// --- Helpers ---

type mockCanonicalizer struct {
	content  []byte
	filename string
	err      error
}

func (m *mockCanonicalizer) Canonicalize(raw []byte, sourceProvider string) ([]byte, string, error) {
	if m.err != nil {
		return nil, "", m.err
	}
	return m.content, m.filename, nil
}
