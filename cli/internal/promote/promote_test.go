package promote

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// initGitRepo creates a temp dir with a git repo, an initial commit, and a remote.
// Returns the repo root path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@example.com"},
		{"git", "-C", dir, "config", "user.name", "Test User"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git setup %v failed: %v\n%s", args, err, out)
		}
	}

	// Create an initial file and commit so we have a HEAD
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit setup %v failed: %v\n%s", args, err, out)
		}
	}

	return dir
}

// --- sharedPath tests (pure function) ---

func TestSharedPath_UniversalType(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:     "my-skill",
		Type:     catalog.Skills, // universal
		Provider: "claude-code",
	}
	got := sharedPath("/repo", item)
	want := filepath.Join("/repo", "skills", "my-skill")
	if got != want {
		t.Errorf("sharedPath() = %q, want %q", got, want)
	}
}

func TestSharedPath_ProviderSpecificType(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:     "my-rule",
		Type:     catalog.Rules, // provider-specific
		Provider: "claude-code",
	}
	got := sharedPath("/repo", item)
	want := filepath.Join("/repo", "rules", "claude-code", "my-rule")
	if got != want {
		t.Errorf("sharedPath() = %q, want %q", got, want)
	}
}

// --- buildCompareURL tests (needs git remote) ---

func TestBuildCompareURL_SSHRemote(t *testing.T) {
	dir := initGitRepo(t)
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:org/repo.git")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	got := buildCompareURL(dir, "feature/branch")
	want := "https://github.com/org/repo/compare/feature/branch?expand=1"
	if got != want {
		t.Errorf("buildCompareURL() = %q, want %q", got, want)
	}
}

func TestBuildCompareURL_HTTPSRemote(t *testing.T) {
	dir := initGitRepo(t)
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin", "https://github.com/org/repo.git")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	got := buildCompareURL(dir, "my-branch")
	want := "https://github.com/org/repo/compare/my-branch?expand=1"
	if got != want {
		t.Errorf("buildCompareURL() = %q, want %q", got, want)
	}
}

func TestBuildCompareURL_HTTPSNoGitSuffix(t *testing.T) {
	dir := initGitRepo(t)
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin", "https://github.com/org/repo")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	got := buildCompareURL(dir, "my-branch")
	want := "https://github.com/org/repo/compare/my-branch?expand=1"
	if got != want {
		t.Errorf("buildCompareURL() = %q, want %q", got, want)
	}
}

func TestBuildCompareURL_NoRemote(t *testing.T) {
	dir := initGitRepo(t)
	// No remote added
	got := buildCompareURL(dir, "branch")
	if got != "" {
		t.Errorf("expected empty string for no remote, got %q", got)
	}
}

func TestBuildCompareURL_NonGitHubRemote(t *testing.T) {
	dir := initGitRepo(t)
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin", "https://gitlab.com/org/repo.git")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	got := buildCompareURL(dir, "branch")
	if got != "" {
		t.Errorf("expected empty string for non-GitHub remote, got %q", got)
	}
}

// --- isTreeDirty tests ---

func TestIsTreeDirty_CleanRepo(t *testing.T) {
	dir := initGitRepo(t)
	dirty, err := isTreeDirty(dir)
	if err != nil {
		t.Fatalf("isTreeDirty() error: %v", err)
	}
	if dirty {
		t.Error("expected clean repo to report not dirty")
	}
}

func TestIsTreeDirty_UntrackedFile(t *testing.T) {
	dir := initGitRepo(t)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644)

	dirty, err := isTreeDirty(dir)
	if err != nil {
		t.Fatalf("isTreeDirty() error: %v", err)
	}
	if !dirty {
		t.Error("expected dirty with untracked file")
	}
}

func TestIsTreeDirty_ModifiedFile(t *testing.T) {
	dir := initGitRepo(t)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed"), 0644)

	dirty, err := isTreeDirty(dir)
	if err != nil {
		t.Fatalf("isTreeDirty() error: %v", err)
	}
	if !dirty {
		t.Error("expected dirty with modified file")
	}
}

func TestIsTreeDirty_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := isTreeDirty(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

// --- detectDefaultBranch tests ---

func TestDetectDefaultBranch_Fallback(t *testing.T) {
	// Without origin/HEAD set, should fall back to "main"
	dir := initGitRepo(t)
	got := detectDefaultBranch(dir)
	if got != "main" {
		t.Errorf("detectDefaultBranch() = %q, want %q", got, "main")
	}
}

// --- gitRun tests ---

func TestGitRun_Success(t *testing.T) {
	dir := initGitRepo(t)
	err := gitRun(dir, "status")
	if err != nil {
		t.Errorf("gitRun(status) should succeed, got: %v", err)
	}
}

func TestGitRun_Failure(t *testing.T) {
	dir := t.TempDir()
	// "git log" in a non-git dir should fail
	err := gitRun(dir, "log")
	if err == nil {
		t.Error("expected error for git command in non-git dir")
	}
}

// --- copyForPromote tests ---

func TestCopyForPromote_CopiesFiles(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Create source files
	os.WriteFile(filepath.Join(src, "rule.md"), []byte("# Rule"), 0644)
	os.MkdirAll(filepath.Join(src, "subdir"), 0755)
	os.WriteFile(filepath.Join(src, "subdir", "nested.txt"), []byte("nested"), 0644)

	err := copyForPromote(src, dst)
	if err != nil {
		t.Fatalf("copyForPromote() error: %v", err)
	}

	// Verify files were copied
	data, err := os.ReadFile(filepath.Join(dst, "rule.md"))
	if err != nil {
		t.Fatalf("rule.md not copied: %v", err)
	}
	if string(data) != "# Rule" {
		t.Errorf("rule.md content = %q, want %q", string(data), "# Rule")
	}

	data, err = os.ReadFile(filepath.Join(dst, "subdir", "nested.txt"))
	if err != nil {
		t.Fatalf("nested.txt not copied: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("nested.txt content = %q, want %q", string(data), "nested")
	}
}

func TestCopyForPromote_ExcludesLLMPrompt(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	os.WriteFile(filepath.Join(src, "rule.md"), []byte("# Rule"), 0644)
	os.WriteFile(filepath.Join(src, "LLM-PROMPT.md"), []byte("scaffold"), 0644)

	err := copyForPromote(src, dst)
	if err != nil {
		t.Fatalf("copyForPromote() error: %v", err)
	}

	// rule.md should exist
	if _, err := os.Stat(filepath.Join(dst, "rule.md")); err != nil {
		t.Error("rule.md should be copied")
	}

	// LLM-PROMPT.md should NOT exist
	if _, err := os.Stat(filepath.Join(dst, "LLM-PROMPT.md")); err == nil {
		t.Error("LLM-PROMPT.md should be excluded from copy")
	}
}

// --- Promote integration test ---

func TestPromote_RejectsDirtyTree(t *testing.T) {
	dir := initGitRepo(t)

	// Create an untracked file to dirty the tree
	os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty"), 0644)

	item := catalog.ContentItem{
		Name:     "test-rule",
		Type:     catalog.Rules,
		Provider: "claude-code",
		Path:     filepath.Join(dir, "rules", "claude-code", "test-rule"),
		Meta: &metadata.Meta{
			ID:   "test-id",
			Name: "test-rule",
			Type: "rules",
		},
	}

	_, err := Promote(dir, item, true)
	if err == nil {
		t.Fatal("expected error for dirty tree")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error should mention uncommitted changes, got: %s", err)
	}
}

func TestPromote_RejectsNilMeta(t *testing.T) {
	dir := initGitRepo(t)
	item := catalog.ContentItem{
		Name: "test-rule",
		Type: catalog.Rules,
	}

	_, err := Promote(dir, item, true)
	if err == nil {
		t.Fatal("expected error for nil meta")
	}
	if !strings.Contains(err.Error(), "no .syllago.yaml") {
		t.Errorf("error should mention missing metadata, got: %s", err)
	}
}

func TestPromote_RejectsValidationFailure(t *testing.T) {
	dir := initGitRepo(t)

	// Item with meta but missing required fields (ID is empty)
	itemDir := filepath.Join(dir, "library", "rules", "test-rule")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# Rule"), 0644)
	// Save an incomplete .syllago.yaml (missing id, name, type)
	os.WriteFile(filepath.Join(itemDir, ".syllago.yaml"), []byte("description: test\n"), 0644)

	// Commit these files so the tree is clean (dirty check runs first)
	for _, args := range [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "add item"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	item := catalog.ContentItem{
		Name:     "test-rule",
		Type:     catalog.Rules,
		Provider: "claude-code",
		Path:     itemDir,
		Meta:     &metadata.Meta{
			// Missing ID, Name, Type — validation should fail
		},
	}

	_, err := Promote(dir, item, true)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error should mention validation, got: %s", err)
	}
}

func TestPromote_FailsAtPush_ButCreatedBranchAndCommit(t *testing.T) {
	dir := initGitRepo(t)

	// Create a valid item in the repo
	itemDir := filepath.Join(dir, "library", "rules", "claude-code", "my-rule")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# My Rule"), 0644)
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte("# README"), 0644)
	meta := &metadata.Meta{ID: metadata.NewID(), Name: "my-rule", Type: "rules"}
	metadata.Save(itemDir, meta)

	// Commit so tree is clean
	for _, args := range [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "add item"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	item := catalog.ContentItem{
		Name:     "my-rule",
		Type:     catalog.Rules,
		Provider: "claude-code",
		Path:     itemDir,
		Meta:     meta,
	}

	// Promote will fail at push (no remote), but exercises branch/copy/commit logic
	_, err := Promote(dir, item, true)
	if err == nil {
		t.Fatal("expected push error (no remote configured)")
	}
	if !strings.Contains(err.Error(), "pushing") {
		t.Errorf("error should mention pushing, got: %s", err)
	}

	// Verify the branch was created (we're still on it since push failed before checkout back)
	out, _ := commandOutput(dir, "git", "branch", "--list", "syllago/promote/rules/my-rule")
	if strings.TrimSpace(out) == "" {
		t.Error("expected branch syllago/promote/rules/my-rule to exist")
	}

	// Verify the file was copied to the shared location
	sharedFile := filepath.Join(dir, "rules", "claude-code", "my-rule", "rule.md")
	if _, err := os.Stat(sharedFile); err != nil {
		t.Errorf("expected shared file to exist at %s", sharedFile)
	}

	// Verify a commit was made
	out, err = commandOutput(dir, "git", "log", "--oneline", "-1")
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if !strings.Contains(out, "Add rules: my-rule") {
		t.Errorf("expected commit message 'Add rules: my-rule', got: %s", out)
	}
}

func TestPromoteToRegistry_ItemAlreadyExists(t *testing.T) {
	// Set up registry cache override
	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	// Create a fake registry clone directory with git
	regName := "test-org/test-reg"
	regDir := filepath.Join(cacheDir, regName)
	os.MkdirAll(regDir, 0755)

	// Init git in the registry dir
	for _, args := range [][]string{
		{"git", "init", regDir},
		{"git", "-C", regDir, "config", "user.email", "test@example.com"},
		{"git", "-C", regDir, "config", "user.name", "Test User"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git setup %v failed: %v\n%s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(regDir, "README.md"), []byte("# reg"), 0644)
	for _, args := range [][]string{
		{"git", "-C", regDir, "add", "."},
		{"git", "-C", regDir, "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.CombinedOutput()
	}

	// Pre-create the item so it already exists
	existingPath := filepath.Join(regDir, "rules", "claude-code", "existing-rule")
	os.MkdirAll(existingPath, 0755)
	os.WriteFile(filepath.Join(existingPath, "rule.md"), []byte("exists"), 0644)

	repoRoot := t.TempDir()
	item := catalog.ContentItem{
		Name:     "existing-rule",
		Type:     catalog.Rules,
		Provider: "claude-code",
		Path:     t.TempDir(),
		Meta:     &metadata.Meta{ID: "test", Name: "existing-rule"},
	}

	_, err := PromoteToRegistry(repoRoot, regName, item, true)
	if err == nil {
		t.Fatal("expected error for item already existing in registry")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention already exists, got: %s", err)
	}
}

func TestPromoteToRegistry_MissingProviderForProviderSpecific(t *testing.T) {
	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	regName := "test-org/test-reg"
	regDir := filepath.Join(cacheDir, regName)
	os.MkdirAll(regDir, 0755)
	// Init a minimal git repo
	exec.Command("git", "init", regDir).Run()

	repoRoot := t.TempDir()
	item := catalog.ContentItem{
		Name:     "no-provider-rule",
		Type:     catalog.Rules,
		Provider: "", // empty provider for provider-specific type
		Path:     t.TempDir(),
		Meta:     &metadata.Meta{ID: "test", Name: "no-provider-rule"},
	}

	_, err := PromoteToRegistry(repoRoot, regName, item, true)
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
	if !strings.Contains(err.Error(), "requires a provider") {
		t.Errorf("error should mention provider requirement, got: %s", err)
	}
}

func TestPromoteToRegistry_RegistryNotCloned(t *testing.T) {
	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	repoRoot := t.TempDir()
	item := catalog.ContentItem{
		Name: "test-rule",
		Type: catalog.Rules,
		Meta: &metadata.Meta{ID: "test", Name: "test-rule"},
	}

	_, err := PromoteToRegistry(repoRoot, "nonexistent/registry", item, true)
	if err == nil {
		t.Fatal("expected error for non-cloned registry")
	}
	if !strings.Contains(err.Error(), "not cloned locally") {
		t.Errorf("error should mention not cloned, got: %s", err)
	}
}

func TestPromoteToRegistry_FailsAtPush(t *testing.T) {
	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	regName := "test-org/test-reg"
	regDir := filepath.Join(cacheDir, regName)

	// Init git in registry dir
	for _, args := range [][]string{
		{"git", "init", regDir},
		{"git", "-C", regDir, "config", "user.email", "test@example.com"},
		{"git", "-C", regDir, "config", "user.name", "Test User"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(regDir, "README.md"), []byte("# reg"), 0644)
	for _, args := range [][]string{
		{"git", "-C", regDir, "add", "."},
		{"git", "-C", regDir, "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.CombinedOutput()
	}

	// Create source item
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("---\nname: test\ndescription: a\n---\n# S"), 0644)

	repoRoot := t.TempDir()
	now := time.Now()
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: regName, URL: "https://github.com/test-org/test-reg", Visibility: "public", VisibilityCheckedAt: &now},
		},
	}
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)
	config.Save(repoRoot, cfg)

	item := catalog.ContentItem{
		Name:     "new-skill",
		Type:     catalog.Skills, // universal — no provider needed in path
		Provider: "claude-code",
		Path:     srcDir,
		Meta:     &metadata.Meta{ID: "test", Name: "new-skill"},
	}

	_, err := PromoteToRegistry(repoRoot, regName, item, true)
	if err == nil {
		t.Fatal("expected push error (no remote)")
	}
	if !strings.Contains(err.Error(), "pushing") {
		t.Errorf("error should mention pushing, got: %s", err)
	}

	// Verify branch was created and content was copied
	out, _ := commandOutput(regDir, "git", "branch", "--list", "syllago/contribute/skills/new-skill")
	if strings.TrimSpace(out) == "" {
		t.Error("expected contribution branch to exist")
	}

	copiedFile := filepath.Join(regDir, "skills", "new-skill", "SKILL.md")
	if _, err := os.Stat(copiedFile); err != nil {
		t.Errorf("expected copied file at %s", copiedFile)
	}
}

// --- Promote with a real remote (full happy path minus gh CLI) ---

func TestPromote_FullFlow_PushSucceeds(t *testing.T) {
	// Create a bare repo to act as origin
	bareDir := t.TempDir()
	exec.Command("git", "init", "--bare", bareDir).Run()

	// Clone it so we have a working repo with origin set up
	dir := filepath.Join(t.TempDir(), "work")
	cmd := exec.Command("git", "clone", bareDir, dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}
	exec.Command("git", "-C", dir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run()

	// Create initial commit on master/main and push
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# repo"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	exec.Command("git", "-C", dir, "push", "-u", "origin", "HEAD").Run()

	// Set up the item to promote
	itemDir := filepath.Join(dir, "library", "skills", "my-skill")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "SKILL.md"), []byte("---\nname: test\ndescription: a\n---\n# S"), 0644)
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte("# README"), 0644)
	meta := &metadata.Meta{ID: metadata.NewID(), Name: "my-skill", Type: "skills"}
	metadata.Save(itemDir, meta)

	// Commit and push so tree is clean
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "add item").Run()
	exec.Command("git", "-C", dir, "push").Run()

	item := catalog.ContentItem{
		Name:     "my-skill",
		Type:     catalog.Skills,
		Provider: "claude-code",
		Path:     itemDir,
		Meta:     meta,
	}

	result, err := Promote(dir, item, true)
	if err != nil {
		t.Fatalf("Promote() error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Branch == "" {
		t.Error("expected non-empty branch name")
	}
	if !strings.HasPrefix(result.Branch, "syllago/promote/skills/my-skill") {
		t.Errorf("branch = %q, want prefix syllago/promote/skills/my-skill", result.Branch)
	}
	// CompareURL should be empty (local bare repo, not github)
	// PRUrl should be empty (no gh CLI in test)
}

func TestPromoteToRegistry_FullFlow_PushSucceeds(t *testing.T) {
	// Create a bare repo to act as origin for the registry
	bareDir := t.TempDir()
	exec.Command("git", "init", "--bare", bareDir).Run()

	// Clone it into the cache dir
	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	regName := "test-org/test-reg"
	regDir := filepath.Join(cacheDir, regName)

	cmd := exec.Command("git", "clone", bareDir, regDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}
	exec.Command("git", "-C", regDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", regDir, "config", "user.name", "Test User").Run()

	// Initial commit
	os.WriteFile(filepath.Join(regDir, "README.md"), []byte("# reg"), 0644)
	exec.Command("git", "-C", regDir, "add", ".").Run()
	exec.Command("git", "-C", regDir, "commit", "-m", "init").Run()
	exec.Command("git", "-C", regDir, "push", "-u", "origin", "HEAD").Run()

	// Create source item
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "rule.md"), []byte("# Rule"), 0644)

	repoRoot := t.TempDir()
	now := time.Now()
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: regName, URL: "https://github.com/test-org/test-reg", Visibility: "public", VisibilityCheckedAt: &now},
		},
	}
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)
	config.Save(repoRoot, cfg)

	item := catalog.ContentItem{
		Name:     "new-rule",
		Type:     catalog.Rules,
		Provider: "claude-code",
		Path:     srcDir,
		Meta:     &metadata.Meta{ID: "test", Name: "new-rule"},
	}

	result, err := PromoteToRegistry(repoRoot, regName, item, true)
	if err != nil {
		t.Fatalf("PromoteToRegistry() error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Branch == "" {
		t.Error("expected non-empty branch name")
	}
	if !strings.HasPrefix(result.Branch, "syllago/contribute/rules/new-rule") {
		t.Errorf("branch = %q, want prefix syllago/contribute/rules/new-rule", result.Branch)
	}
}

// --- sanitizeBundledScripts tests ---

func TestSanitizeBundledScripts_StripsAbsolutePaths(t *testing.T) {
	t.Parallel()
	m := &metadata.Meta{
		ID:   "test-id",
		Name: "test-hook",
		BundledScripts: []metadata.BundledScriptMeta{
			{OriginalPath: "/home/user/.claude/hooks/lint.sh", Filename: "lint.sh"},
			{OriginalPath: "/var/data/scripts/format.py", Filename: "format.py"},
			{OriginalPath: "already-filename.sh", Filename: "already-filename.sh"},
			{OriginalPath: "", Filename: "no-path.sh"},
		},
	}

	sanitizeBundledScripts(m)

	tests := []struct {
		idx  int
		want string
	}{
		{0, "lint.sh"},
		{1, "format.py"},
		{2, "already-filename.sh"},
		{3, ""},
	}
	for _, tt := range tests {
		got := m.BundledScripts[tt.idx].OriginalPath
		if got != tt.want {
			t.Errorf("BundledScripts[%d].OriginalPath = %q, want %q", tt.idx, got, tt.want)
		}
	}
}

func TestSanitizeBundledScripts_NilScripts(t *testing.T) {
	t.Parallel()
	m := &metadata.Meta{ID: "test-id", Name: "no-scripts"}
	// Should not panic.
	sanitizeBundledScripts(m)
}

func TestCopyForPromote_SanitizesOriginalPath(t *testing.T) {
	// Integration test: verify that after copyForPromote + metadata save,
	// the .syllago.yaml in the destination has sanitized paths.
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Create source with .syllago.yaml containing absolute OriginalPath.
	m := &metadata.Meta{
		ID:   metadata.NewID(),
		Name: "my-hook",
		Type: "hooks",
		BundledScripts: []metadata.BundledScriptMeta{
			{OriginalPath: "/home/user/.claude/hooks/lint.sh", Filename: "lint.sh"},
		},
	}
	metadata.Save(src, m)
	os.WriteFile(filepath.Join(src, "hook.sh"), []byte("#!/bin/sh\necho hi"), 0644)

	// Simulate what Promote does: copy then sanitize metadata.
	if err := copyForPromote(src, dst); err != nil {
		t.Fatalf("copyForPromote: %v", err)
	}
	destMeta := *m
	sanitizeBundledScripts(&destMeta)
	if err := metadata.Save(dst, &destMeta); err != nil {
		t.Fatalf("metadata.Save: %v", err)
	}

	// Read back the metadata from the destination.
	loaded, err := metadata.Load(dst)
	if err != nil {
		t.Fatalf("metadata.Load: %v", err)
	}
	if len(loaded.BundledScripts) != 1 {
		t.Fatalf("expected 1 bundled script, got %d", len(loaded.BundledScripts))
	}
	if loaded.BundledScripts[0].OriginalPath != "lint.sh" {
		t.Errorf("OriginalPath = %q, want %q", loaded.BundledScripts[0].OriginalPath, "lint.sh")
	}
}

// --- detectDefaultBranch with origin/HEAD set ---

func TestDetectDefaultBranch_WithOriginHead(t *testing.T) {
	dir := initGitRepo(t)

	// Create a bare repo to serve as origin
	bareDir := t.TempDir()
	exec.Command("git", "init", "--bare", bareDir).Run()

	// Add it as origin and push
	exec.Command("git", "-C", dir, "remote", "add", "origin", bareDir).Run()
	exec.Command("git", "-C", dir, "push", "-u", "origin", "master").Run()

	// Set origin/HEAD
	exec.Command("git", "-C", dir, "remote", "set-head", "origin", "master").Run()

	got := detectDefaultBranch(dir)
	if got != "master" {
		t.Errorf("detectDefaultBranch() = %q, want %q", got, "master")
	}
}
