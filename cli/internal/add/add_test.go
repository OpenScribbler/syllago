package add

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestBuildLibraryIndex_EmptyDir(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		t.Fatalf("BuildLibraryIndex on empty dir: %v", err)
	}
	if len(idx) != 0 {
		t.Errorf("expected empty index, got %d entries", len(idx))
	}
}

func TestBuildLibraryIndex_UniversalItem(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()

	// Create a skills item with metadata.
	skillDir := filepath.Join(globalDir, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	now := nowPtr()
	meta := &metadata.Meta{
		ID:         metadata.NewID(),
		Name:       "my-skill",
		SourceHash: "sha256:abc123",
		AddedAt:    now,
	}
	if err := metadata.Save(skillDir, meta); err != nil {
		t.Fatalf("Save: %v", err)
	}

	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		t.Fatalf("BuildLibraryIndex: %v", err)
	}

	key := "skills/my-skill"
	m, ok := idx[key]
	if !ok {
		t.Fatalf("expected key %q in index, keys: %v", key, indexKeys(idx))
	}
	if m.SourceHash != "sha256:abc123" {
		t.Errorf("SourceHash: got %q, want %q", m.SourceHash, "sha256:abc123")
	}
}

func TestBuildLibraryIndex_ProviderSpecificItem(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()

	ruleDir := filepath.Join(globalDir, "rules", "claude-code", "security")
	if err := os.MkdirAll(ruleDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	meta := &metadata.Meta{
		ID:         metadata.NewID(),
		Name:       "security",
		SourceHash: "sha256:def456",
	}
	if err := metadata.Save(ruleDir, meta); err != nil {
		t.Fatalf("Save: %v", err)
	}

	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		t.Fatalf("BuildLibraryIndex: %v", err)
	}

	key := "rules/claude-code/security"
	m, ok := idx[key]
	if !ok {
		t.Fatalf("expected key %q in index", key)
	}
	if m.SourceHash != "sha256:def456" {
		t.Errorf("SourceHash: got %q", m.SourceHash)
	}
}

func TestBuildLibraryIndex_ItemWithNoMetadata(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()

	// Create a skills directory with no .syllago.yaml inside.
	skillDir := filepath.Join(globalDir, "skills", "bare-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		t.Fatalf("BuildLibraryIndex should not error on missing metadata: %v", err)
	}

	key := "skills/bare-skill"
	m, ok := idx[key]
	if !ok {
		t.Fatalf("expected key %q in index (nil value), keys: %v", key, indexKeys(idx))
	}
	if m != nil {
		t.Errorf("expected nil Meta for item with no .syllago.yaml, got %+v", m)
	}
}

func TestSourceHash_Deterministic(t *testing.T) {
	t.Parallel()
	content := []byte("# Security rule\nDon't trust user input.")
	h1 := sourceHash(content)
	h2 := sourceHash(content)
	if h1 != h2 {
		t.Errorf("sourceHash is not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) == 0 || h1[:7] != "sha256:" {
		t.Errorf("expected sha256: prefix, got %q", h1)
	}
}

func TestSourceHash_DifferentContent(t *testing.T) {
	t.Parallel()
	h1 := sourceHash([]byte("content A"))
	h2 := sourceHash([]byte("content B"))
	if h1 == h2 {
		t.Error("different content should produce different hashes")
	}
}

func TestLibraryKey_Universal(t *testing.T) {
	t.Parallel()
	got := libraryKey(catalog.Skills, "claude-code", "my-skill")
	if got != "skills/my-skill" {
		t.Errorf("got %q, want %q", got, "skills/my-skill")
	}
}

func TestLibraryKey_ProviderSpecific(t *testing.T) {
	t.Parallel()
	got := libraryKey(catalog.Rules, "claude-code", "security")
	if got != "rules/claude-code/security" {
		t.Errorf("got %q, want %q", got, "rules/claude-code/security")
	}
}

func TestItemStatusString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status ItemStatus
		want   string
	}{
		{StatusNew, "new"},
		{StatusInLibrary, "in library"},
		{StatusOutdated, "in library, outdated"},
	}
	for _, tc := range tests {
		if got := tc.status.String(); got != tc.want {
			t.Errorf("ItemStatus(%d).String() = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestAddItems_NewItem(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	globalDir := t.TempDir()

	// Create source file.
	content := []byte("# Security rule")
	srcPath := filepath.Join(tmp, "security.md")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items := []DiscoveryItem{
		{Name: "security", Type: catalog.Rules, Path: srcPath, Status: StatusNew},
	}
	opts := AddOptions{Provider: "claude-code"}
	results := AddItems(items, opts, globalDir, nil, "syllago-test")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != AddStatusAdded {
		t.Errorf("expected AddStatusAdded, got %v", results[0].Status)
	}

	// Verify file was written.
	destDir := filepath.Join(globalDir, "rules", "claude-code", "security")
	if _, err := os.Stat(filepath.Join(destDir, "rule.md")); err != nil {
		t.Errorf("rule.md not written: %v", err)
	}

	// Verify metadata has source_hash.
	meta, err := metadata.Load(destDir)
	if err != nil || meta == nil {
		t.Fatalf("metadata load failed: %v", err)
	}
	if meta.SourceHash == "" {
		t.Error("expected source_hash to be set")
	}
	if !strings.HasPrefix(meta.SourceHash, "sha256:") {
		t.Errorf("expected sha256: prefix, got %q", meta.SourceHash)
	}
}

func TestAddItems_UpToDate_Skipped(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()

	content := []byte("# Rule content")
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "my-rule.md")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Pre-populate library with matching hash.
	destDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	meta := &metadata.Meta{
		ID:         metadata.NewID(),
		Name:       "my-rule",
		SourceHash: sourceHash(content),
	}
	if err := metadata.Save(destDir, meta); err != nil {
		t.Fatalf("Save: %v", err)
	}

	items := []DiscoveryItem{
		{Name: "my-rule", Type: catalog.Rules, Path: srcPath, Status: StatusInLibrary},
	}
	results := AddItems(items, AddOptions{Provider: "claude-code"}, globalDir, nil, "syllago-test")

	if results[0].Status != AddStatusUpToDate {
		t.Errorf("expected AddStatusUpToDate, got %v", results[0].Status)
	}
}

func TestAddItems_Outdated_SkippedWithoutForce(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "rule.md")
	if err := os.WriteFile(srcPath, []byte("new content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items := []DiscoveryItem{
		{Name: "rule", Type: catalog.Rules, Path: srcPath, Status: StatusOutdated},
	}
	results := AddItems(items, AddOptions{Provider: "claude-code"}, globalDir, nil, "test")
	if results[0].Status != AddStatusSkipped {
		t.Errorf("expected AddStatusSkipped, got %v", results[0].Status)
	}
}

func TestAddItems_Outdated_UpdatedWithForce(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "rule.md")
	if err := os.WriteFile(srcPath, []byte("updated content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items := []DiscoveryItem{
		{Name: "rule", Type: catalog.Rules, Path: srcPath, Status: StatusOutdated},
	}
	results := AddItems(items, AddOptions{Provider: "claude-code", Force: true}, globalDir, nil, "test")
	if results[0].Status != AddStatusUpdated {
		t.Errorf("expected AddStatusUpdated, got %v", results[0].Status)
	}
}

func TestAddItems_DryRun_NoWrite(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "rule.md")
	if err := os.WriteFile(srcPath, []byte("# Content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items := []DiscoveryItem{
		{Name: "rule", Type: catalog.Rules, Path: srcPath, Status: StatusNew},
	}
	results := AddItems(items, AddOptions{Provider: "claude-code", DryRun: true}, globalDir, nil, "test")
	if results[0].Status != AddStatusAdded {
		t.Errorf("expected AddStatusAdded in dry-run, got %v", results[0].Status)
	}

	// Verify nothing was actually written.
	entries, _ := os.ReadDir(globalDir)
	if len(entries) != 0 {
		t.Errorf("dry-run should not write anything, found %d entries", len(entries))
	}
}

func TestAddItems_SourcePreservation(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()
	// Non-.md source file should be preserved in .source/
	srcPath := filepath.Join(srcDir, "my-rule.mdc")
	if err := os.WriteFile(srcPath, []byte("# Cursor rule"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items := []DiscoveryItem{
		{Name: "my-rule", Type: catalog.Rules, Path: srcPath, Status: StatusNew},
	}
	results := AddItems(items, AddOptions{Provider: "cursor"}, globalDir, nil, "test")
	if results[0].Status != AddStatusAdded {
		t.Errorf("expected AddStatusAdded, got %v", results[0].Status)
	}

	destDir := filepath.Join(globalDir, "rules", "cursor", "my-rule")
	sourceCopy := filepath.Join(destDir, ".source", "my-rule.mdc")
	if _, err := os.Stat(sourceCopy); err != nil {
		t.Errorf(".source/ copy not written: %v", err)
	}

	// Metadata should have has_source=true.
	meta, err := metadata.Load(destDir)
	if err != nil || meta == nil {
		t.Fatalf("metadata load failed: %v", err)
	}
	if !meta.HasSource {
		t.Error("expected HasSource=true for non-.md source")
	}
}

func TestAddItems_MDSource_NoPreservation(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "my-rule.md")
	if err := os.WriteFile(srcPath, []byte("# Rule"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items := []DiscoveryItem{
		{Name: "my-rule", Type: catalog.Rules, Path: srcPath, Status: StatusNew},
	}
	AddItems(items, AddOptions{Provider: "claude-code"}, globalDir, nil, "test")

	destDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
	if _, err := os.Stat(filepath.Join(destDir, ".source")); err == nil {
		t.Error("expected no .source/ directory for .md source file")
	}
}

func TestAddItems_UniversalType_NoProviderDir(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "my-skill.md")
	if err := os.WriteFile(srcPath, []byte("# Skill"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items := []DiscoveryItem{
		{Name: "my-skill", Type: catalog.Skills, Path: srcPath, Status: StatusNew},
	}
	AddItems(items, AddOptions{Provider: "claude-code"}, globalDir, nil, "test")

	// Universal types go into <globalDir>/skills/<name>/ (no provider dir).
	destDir := filepath.Join(globalDir, "skills", "my-skill")
	if _, err := os.Stat(filepath.Join(destDir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not written at universal path: %v", err)
	}
}

func TestAddItems_InLibrary_ForceOverwrites(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "rule.md")
	if err := os.WriteFile(srcPath, []byte("updated"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items := []DiscoveryItem{
		{Name: "rule", Type: catalog.Rules, Path: srcPath, Status: StatusInLibrary},
	}
	results := AddItems(items, AddOptions{Provider: "claude-code", Force: true}, globalDir, nil, "test")
	if results[0].Status != AddStatusUpdated {
		t.Errorf("expected AddStatusUpdated with --force on InLibrary, got %v", results[0].Status)
	}
}

func TestDiscoverItemsAtPath_SingleFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(srcPath, []byte("# Rules"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items := discoverItemsAtPath(srcPath)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].name != "CLAUDE" {
		t.Errorf("expected name %q, got %q", "CLAUDE", items[0].name)
	}
	if items[0].path != srcPath {
		t.Errorf("expected path %q, got %q", srcPath, items[0].path)
	}
}

func TestDiscoverItemsAtPath_DirectoryWithFiles(t *testing.T) {
	t.Parallel()
	// Simulate rules/ directory with single-file items.
	dir := t.TempDir()
	for _, name := range []string{"security.md", "testing.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	items := discoverItemsAtPath(dir)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	// Items should use filename sans extension as name.
	names := map[string]bool{}
	for _, item := range items {
		names[item.name] = true
	}
	if !names["security"] || !names["testing"] {
		t.Errorf("expected names {security, testing}, got %v", names)
	}
}

func TestDiscoverItemsAtPath_DirectoryWithSubdirs(t *testing.T) {
	t.Parallel()
	// Simulate skills/ directory with directory-based items.
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	contentPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(contentPath, []byte("# Skill"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	items := discoverItemsAtPath(dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].name != "my-skill" {
		t.Errorf("expected name %q, got %q", "my-skill", items[0].name)
	}
	if items[0].path != contentPath {
		t.Errorf("expected path to content file, got %q", items[0].path)
	}
}

func TestDiscoverItemsAtPath_SkipsHiddenEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Hidden file should be skipped.
	os.WriteFile(filepath.Join(dir, ".hidden.md"), []byte("hidden"), 0644)
	// Hidden directory should be skipped.
	hiddenDir := filepath.Join(dir, ".config")
	os.MkdirAll(hiddenDir, 0755)
	os.WriteFile(filepath.Join(hiddenDir, "file.md"), []byte("content"), 0644)
	// Visible file should be found.
	os.WriteFile(filepath.Join(dir, "visible.md"), []byte("visible"), 0644)

	items := discoverItemsAtPath(dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 item (hidden skipped), got %d", len(items))
	}
	if items[0].name != "visible" {
		t.Errorf("expected %q, got %q", "visible", items[0].name)
	}
}

func TestDiscoverItemsAtPath_EmptySubdirSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Empty subdirectory — no content file to find.
	os.MkdirAll(filepath.Join(dir, "empty-skill"), 0755)

	items := discoverItemsAtPath(dir)
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty subdir, got %d", len(items))
	}
}

func TestDiscoverItemsAtPath_SymlinkToDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a target directory with content.
	target := filepath.Join(dir, "target-skill")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create a parent directory with a symlink to the target.
	parent := filepath.Join(dir, "skills")
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	symlink := filepath.Join(parent, "linked-skill")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	items := discoverItemsAtPath(parent)
	if len(items) != 1 {
		t.Fatalf("expected 1 item from symlink, got %d", len(items))
	}
	if items[0].name != "linked-skill" {
		t.Errorf("expected name %q (symlink name), got %q", "linked-skill", items[0].name)
	}
	// Path should point to the content file inside the symlinked directory.
	if !strings.HasSuffix(items[0].path, "SKILL.md") {
		t.Errorf("expected path to end with SKILL.md, got %q", items[0].path)
	}
}

func TestFindContentFile_SkipsHidden(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".syllago.yaml"), []byte("id: test"), 0644)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0644)

	got := findContentFile(dir)
	if !strings.HasSuffix(got, "SKILL.md") {
		t.Errorf("expected SKILL.md (not hidden .syllago.yaml), got %q", got)
	}
}

func TestFindContentFile_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got := findContentFile(dir)
	if got != "" {
		t.Errorf("expected empty string for empty dir, got %q", got)
	}
}

func TestAddItems_MultiFileSkill(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()

	// Create a multi-file skill: SKILL.md + workflows/ subdirectory.
	skillDir := filepath.Join(srcDir, "atlassian")
	os.MkdirAll(filepath.Join(skillDir, "workflows", "jira"), 0755)
	os.MkdirAll(filepath.Join(skillDir, "workflows", "confluence"), 0755)

	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Atlassian Skill"), 0644)
	os.WriteFile(filepath.Join(skillDir, "workflows", "jira", "create-issue.md"), []byte("# Create Issue"), 0644)
	os.WriteFile(filepath.Join(skillDir, "workflows", "jira", "search-issues.md"), []byte("# Search Issues"), 0644)
	os.WriteFile(filepath.Join(skillDir, "workflows", "confluence", "view-page.md"), []byte("# View Page"), 0644)
	os.WriteFile(filepath.Join(skillDir, "workflows", "search-atlassian.md"), []byte("# Search"), 0644)

	items := []DiscoveryItem{
		{
			Name:      "atlassian",
			Type:      catalog.Skills,
			Path:      filepath.Join(skillDir, "SKILL.md"),
			SourceDir: skillDir,
			Status:    StatusNew,
		},
	}
	results := AddItems(items, AddOptions{Provider: "claude-code"}, globalDir, nil, "test")

	if len(results) != 1 || results[0].Status != AddStatusAdded {
		t.Fatalf("expected AddStatusAdded, got %v", results[0].Status)
	}

	// Verify all files were copied.
	destDir := filepath.Join(globalDir, "skills", "atlassian")

	wantFiles := []string{
		"SKILL.md",
		"workflows/jira/create-issue.md",
		"workflows/jira/search-issues.md",
		"workflows/confluence/view-page.md",
		"workflows/search-atlassian.md",
	}
	for _, f := range wantFiles {
		full := filepath.Join(destDir, f)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected file %s to exist: %v", f, err)
		}
	}

	// Verify content is preserved.
	data, err := os.ReadFile(filepath.Join(destDir, "workflows", "jira", "create-issue.md"))
	if err != nil {
		t.Fatalf("reading workflow file: %v", err)
	}
	if string(data) != "# Create Issue" {
		t.Errorf("workflow content mismatch: got %q", string(data))
	}
}

func TestAddItems_MultiFileSkill_HiddenFilesExcluded(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()

	skillDir := filepath.Join(srcDir, "my-skill")
	os.MkdirAll(filepath.Join(skillDir, ".hidden-dir"), 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0644)
	os.WriteFile(filepath.Join(skillDir, "ref.md"), []byte("# Ref"), 0644)
	os.WriteFile(filepath.Join(skillDir, ".hidden-file"), []byte("hidden"), 0644)
	os.WriteFile(filepath.Join(skillDir, ".hidden-dir", "secret.md"), []byte("secret"), 0644)

	items := []DiscoveryItem{
		{
			Name:      "my-skill",
			Type:      catalog.Skills,
			Path:      filepath.Join(skillDir, "SKILL.md"),
			SourceDir: skillDir,
			Status:    StatusNew,
		},
	}
	AddItems(items, AddOptions{Provider: "claude-code"}, globalDir, nil, "test")

	destDir := filepath.Join(globalDir, "skills", "my-skill")

	// ref.md should be copied.
	if _, err := os.Stat(filepath.Join(destDir, "ref.md")); err != nil {
		t.Error("expected ref.md to be copied")
	}
	// Hidden files/dirs should NOT be copied.
	if _, err := os.Stat(filepath.Join(destDir, ".hidden-file")); err == nil {
		t.Error("hidden file should not be copied")
	}
	if _, err := os.Stat(filepath.Join(destDir, ".hidden-dir")); err == nil {
		t.Error("hidden directory should not be copied")
	}
}

func TestDiscoverItemsAtPath_DirectoryWithSubdirs_SetsSourceDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(filepath.Join(skillDir, "workflows"), 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0644)
	os.WriteFile(filepath.Join(skillDir, "workflows", "flow.md"), []byte("# Flow"), 0644)

	items := discoverItemsAtPath(dir)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].sourceDir != skillDir {
		t.Errorf("expected sourceDir %q, got %q", skillDir, items[0].sourceDir)
	}
}

func TestDiscoverItemsAtPath_SingleFile_EmptySourceDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "rule.md")
	os.WriteFile(srcPath, []byte("# Rule"), 0644)

	items := discoverItemsAtPath(srcPath)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].sourceDir != "" {
		t.Errorf("expected empty sourceDir for single file, got %q", items[0].sourceDir)
	}
}

func TestWriteItem_StripsMaliciousAnalyzerFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create source dir with a malicious .syllago.yaml
	srcDir := filepath.Join(dir, "src", "rules", "evil-rule")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "rule.md"), []byte("# Rule"), 0644)
	// Malicious metadata with attacker-set confidence
	os.WriteFile(filepath.Join(srcDir, metadata.FileName), []byte(
		"id: evil\nname: evil-rule\nconfidence: 0.99\ndetection_method: user-directed\n"), 0644)

	destDir := filepath.Join(dir, "content")
	item := DiscoveryItem{
		Name:      "evil-rule",
		Type:      catalog.Rules,
		Path:      filepath.Join(srcDir, "rule.md"),
		SourceDir: srcDir,
		Status:    StatusNew,
		// No Confidence set on the item itself — no legitimate detection
	}
	result := writeItem(item, AddOptions{}, destDir, nil, "syllago-test")
	if result.Status == AddStatusError {
		t.Fatalf("writeItem error: %v", result.Error)
	}

	destItemDir := filepath.Join(destDir, "rules", "evil-rule")
	m, err := metadata.Load(destItemDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Confidence != 0 {
		t.Errorf("Confidence should be stripped, got %v", m.Confidence)
	}
	if m.DetectionMethod != "" {
		t.Errorf("DetectionMethod should be stripped, got %q", m.DetectionMethod)
	}
}

func TestAddItems_PersistsConfidenceMetadata(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	contentDir := filepath.Join(dir, "content")
	srcDir := filepath.Join(dir, "src", "rules", "my-rule")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "rule.md"), []byte("# My Rule\nContent here."), 0644)

	item := DiscoveryItem{
		Name:            "my-rule",
		Type:            catalog.Rules,
		Path:            filepath.Join(srcDir, "rule.md"),
		SourceDir:       srcDir,
		Status:          StatusNew,
		Confidence:      0.75,
		DetectionSource: "content-signal",
		DetectionMethod: "automatic",
	}

	results := AddItems([]DiscoveryItem{item}, AddOptions{}, contentDir, nil, "syllago-test")
	if results[0].Status == AddStatusError {
		t.Fatalf("AddItems error: %v", results[0].Error)
	}

	destDir := filepath.Join(contentDir, "rules", "my-rule")
	m, err := metadata.Load(destDir)
	if err != nil {
		t.Fatalf("Load metadata: %v", err)
	}
	if m.Confidence != 0.75 {
		t.Errorf("Confidence: got %v, want 0.75", m.Confidence)
	}
	if m.DetectionSource != "content-signal" {
		t.Errorf("DetectionSource: got %q, want content-signal", m.DetectionSource)
	}
	if m.DetectionMethod != "automatic" {
		t.Errorf("DetectionMethod: got %q, want automatic", m.DetectionMethod)
	}
}

func TestWriteItem_StripsAllAnalyzerFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src", "rules", "evil")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "rule.md"), []byte("# Evil"), 0644)
	os.WriteFile(filepath.Join(srcDir, metadata.FileName), []byte(
		"id: evil\nname: evil\nconfidence: 0.99\ndetection_source: content-signal\ndetection_method: user-directed\n"), 0644)

	destDir := filepath.Join(dir, "content")
	item := DiscoveryItem{
		Name:      "evil",
		Type:      catalog.Rules,
		Path:      filepath.Join(srcDir, "rule.md"),
		SourceDir: srcDir,
		Status:    StatusNew,
	}
	result := writeItem(item, AddOptions{}, destDir, nil, "syllago-test")
	if result.Status == AddStatusError {
		t.Fatalf("writeItem error: %v", result.Error)
	}

	m, err := metadata.Load(filepath.Join(destDir, "rules", "evil"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Confidence != 0 {
		t.Errorf("Confidence should be 0, got %v", m.Confidence)
	}
	if m.DetectionSource != "" {
		t.Errorf("DetectionSource should be empty, got %q", m.DetectionSource)
	}
	if m.DetectionMethod != "" {
		t.Errorf("DetectionMethod should be empty, got %q", m.DetectionMethod)
	}
}

// Bug C regression: copySupportingFiles must not follow symlinks. Without
// this guard, a symlink-to-directory in the source tree caused the walk to
// encounter an entry where d.IsDir() reports false (WalkDir doesn't follow
// symlinks) but os.ReadFile DOES follow — which then errors with
// "is a directory" and aborts the whole add operation.
func TestCopySupportingFiles_SkipsSymlinkToDirectory(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Primary file (skipped by copySupportingFiles because rel == primaryFilename).
	primary := "SKILL.md"
	if err := os.WriteFile(filepath.Join(srcDir, primary), []byte("body"), 0644); err != nil {
		t.Fatal(err)
	}
	// A regular supporting file (should be copied).
	if err := os.WriteFile(filepath.Join(srcDir, "notes.md"), []byte("notes"), 0644); err != nil {
		t.Fatal(err)
	}
	// External directory that the symlink will point to (outside srcDir).
	externalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(externalDir, "external.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// Symlink from srcDir/linked-dir -> externalDir. This mimics the real-world
	// pattern that broke ~/.config/pai/history/learnings -> /mnt/c/.../learnings.
	if err := os.Symlink(externalDir, filepath.Join(srcDir, "linked-dir")); err != nil {
		t.Skipf("platform doesn't support symlinks: %v", err)
	}

	if err := copySupportingFiles(srcDir, destDir, primary); err != nil {
		t.Fatalf("copySupportingFiles: %v", err)
	}

	// Regular supporting file must have been copied.
	if _, err := os.Stat(filepath.Join(destDir, "notes.md")); err != nil {
		t.Errorf("notes.md should have been copied: %v", err)
	}
	// Symlink target content must NOT have been copied.
	if _, err := os.Stat(filepath.Join(destDir, "linked-dir")); err == nil {
		t.Error("symlink should not be materialized in destDir (copy must skip symlinks to avoid importing external content)")
	}
	if _, err := os.Stat(filepath.Join(destDir, "linked-dir", "external.txt")); err == nil {
		t.Error("files under a symlinked directory must not be copied")
	}
}

// Bug C related: a symlink to a regular file must also be skipped — otherwise
// os.ReadFile follows the link and copies content whose origin is outside the
// item's source directory.
func TestCopySupportingFiles_SkipsSymlinkToFile(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	destDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("body"), 0644); err != nil {
		t.Fatal(err)
	}
	externalFile := filepath.Join(t.TempDir(), "external.md")
	if err := os.WriteFile(externalFile, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalFile, filepath.Join(srcDir, "linked-file.md")); err != nil {
		t.Skipf("platform doesn't support symlinks: %v", err)
	}

	if err := copySupportingFiles(srcDir, destDir, "SKILL.md"); err != nil {
		t.Fatalf("copySupportingFiles: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "linked-file.md")); err == nil {
		t.Error("symlink-to-file should not be copied into destDir")
	}
}

// helpers

func nowPtr() *time.Time {
	now := time.Now().UTC()
	return &now
}

func indexKeys(idx LibraryIndex) []string {
	var keys []string
	for k := range idx {
		keys = append(keys, k)
	}
	return keys
}
