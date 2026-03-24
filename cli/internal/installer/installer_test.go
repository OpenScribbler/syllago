package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// testProvider returns a provider stub for installer tests.
// It supports Rules (filesystem) and Hooks/MCP (JSON merge).
func testProvider(slug string) provider.Provider {
	return provider.Provider{
		Name:      "Test Provider",
		Slug:      slug,
		ConfigDir: ".testprovider",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			switch ct {
			case catalog.Rules:
				return filepath.Join(homeDir, ".testprovider", "rules")
			case catalog.Skills:
				return filepath.Join(homeDir, ".testprovider", "skills")
			case catalog.Agents:
				return filepath.Join(homeDir, ".testprovider", "agents")
			case catalog.Hooks:
				return provider.JSONMergeSentinel
			case catalog.MCP:
				return provider.JSONMergeSentinel
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			switch ct {
			case catalog.Rules, catalog.Skills, catalog.Agents, catalog.Hooks, catalog.MCP:
				return true
			}
			return false
		},
	}
}

// testProviderProjectScope returns a provider that returns ProjectScopeSentinel for rules.
func testProviderProjectScope(slug string) provider.Provider {
	return provider.Provider{
		Name:      "Project Scope Provider",
		Slug:      slug,
		ConfigDir: ".projectprov",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			switch ct {
			case catalog.Rules:
				return provider.ProjectScopeSentinel
			case catalog.Skills:
				return filepath.Join(homeDir, ".projectprov", "skills")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules || ct == catalog.Skills
		},
	}
}

func TestResolveTargetWithBase(t *testing.T) {
	t.Parallel()

	prov := testProvider("test")

	t.Run("rules item resolves to base path", func(t *testing.T) {
		t.Parallel()
		item := catalog.ContentItem{
			Name: "my-rule",
			Type: catalog.Rules,
			Path: "/repo/rules/test/my-rule",
		}
		got, err := resolveTargetWithBase(item, prov, "/home/user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "/home/user/.testprovider/rules/my-rule"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("agents item appends .md", func(t *testing.T) {
		t.Parallel()
		item := catalog.ContentItem{
			Name: "my-agent",
			Type: catalog.Agents,
			Path: "/repo/agents/test/my-agent",
		}
		got, err := resolveTargetWithBase(item, prov, "/home/user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "/home/user/.testprovider/agents/my-agent.md"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		t.Parallel()
		item := catalog.ContentItem{
			Name: "my-cmd",
			Type: catalog.Commands,
			Path: "/repo/commands/test/my-cmd",
		}
		_, err := resolveTargetWithBase(item, prov, "/home/user")
		if err == nil {
			t.Fatal("expected error for unsupported type")
		}
	})

	t.Run("JSON merge type returns error", func(t *testing.T) {
		t.Parallel()
		item := catalog.ContentItem{
			Name: "my-hook",
			Type: catalog.Hooks,
			Path: "/repo/hooks/test/my-hook",
		}
		_, err := resolveTargetWithBase(item, prov, "/home/user")
		if err == nil {
			t.Fatal("expected error for JSON merge type")
		}
	})

	t.Run("project scope returns error", func(t *testing.T) {
		t.Parallel()
		prov := testProviderProjectScope("projprov")
		item := catalog.ContentItem{
			Name: "my-rule",
			Type: catalog.Rules,
			Path: "/repo/rules/projprov/my-rule",
		}
		_, err := resolveTargetWithBase(item, prov, "/home/user")
		if err == nil {
			t.Fatal("expected error for project-scoped type")
		}
	})
}

func TestCheckStatus_FilesystemItems(t *testing.T) {
	// Not parallel — uses t.Setenv
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)

	// Set HOME so resolveTarget works predictably
	t.Setenv("HOME", tmp)

	prov := testProvider("test")

	t.Run("not installed returns StatusNotInstalled", func(t *testing.T) {
		item := catalog.ContentItem{
			Name: "missing-rule",
			Type: catalog.Rules,
			Path: filepath.Join(repoRoot, "rules", "test", "missing-rule"),
		}
		os.MkdirAll(item.Path, 0755)
		status := CheckStatus(item, prov, repoRoot)
		if status != StatusNotInstalled {
			t.Errorf("expected StatusNotInstalled, got %v", status)
		}
	})

	t.Run("symlink to repo returns StatusInstalled", func(t *testing.T) {
		sourcePath := filepath.Join(repoRoot, "rules", "test", "linked-rule")
		os.MkdirAll(sourcePath, 0755)
		item := catalog.ContentItem{
			Name: "linked-rule",
			Type: catalog.Rules,
			Path: sourcePath,
		}
		targetPath := filepath.Join(tmp, ".testprovider", "rules", "linked-rule")
		os.MkdirAll(filepath.Dir(targetPath), 0755)
		os.Symlink(sourcePath, targetPath)

		status := CheckStatus(item, prov, repoRoot)
		if status != StatusInstalled {
			t.Errorf("expected StatusInstalled for symlink, got %v", status)
		}
	})

	t.Run("regular file (copy) returns StatusInstalled", func(t *testing.T) {
		item := catalog.ContentItem{
			Name: "copied-rule",
			Type: catalog.Rules,
			Path: filepath.Join(repoRoot, "rules", "test", "copied-rule"),
		}
		os.MkdirAll(item.Path, 0755)
		targetPath := filepath.Join(tmp, ".testprovider", "rules", "copied-rule")
		os.MkdirAll(filepath.Dir(targetPath), 0755)
		os.WriteFile(targetPath, []byte("# Copied"), 0644)

		status := CheckStatus(item, prov, repoRoot)
		if status != StatusInstalled {
			t.Errorf("expected StatusInstalled for copy, got %v", status)
		}
	})

	t.Run("unsupported type returns StatusNotAvailable", func(t *testing.T) {
		item := catalog.ContentItem{
			Name: "cmd",
			Type: catalog.Commands,
			Path: filepath.Join(repoRoot, "commands", "test", "cmd"),
		}
		status := CheckStatus(item, prov, repoRoot)
		if status != StatusNotAvailable {
			t.Errorf("expected StatusNotAvailable, got %v", status)
		}
	})

	t.Run("registry paths are checked for symlinks", func(t *testing.T) {
		regRoot := filepath.Join(tmp, "registry-cache")
		regSource := filepath.Join(regRoot, "rules", "test", "reg-rule")
		os.MkdirAll(regSource, 0755)
		item := catalog.ContentItem{
			Name: "reg-rule",
			Type: catalog.Rules,
			Path: regSource,
		}
		targetPath := filepath.Join(tmp, ".testprovider", "rules", "reg-rule")
		os.MkdirAll(filepath.Dir(targetPath), 0755)
		os.Symlink(regSource, targetPath)

		status := CheckStatus(item, prov, repoRoot, regRoot)
		if status != StatusInstalled {
			t.Errorf("expected StatusInstalled with registry path, got %v", status)
		}
	})
}

func TestInstall_SymlinkMethod(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)
	t.Setenv("HOME", tmp)

	prov := testProvider("test")

	sourcePath := filepath.Join(repoRoot, "rules", "test", "install-rule")
	os.MkdirAll(sourcePath, 0755)
	os.WriteFile(filepath.Join(sourcePath, "rule.md"), []byte("# My Rule"), 0644)

	item := catalog.ContentItem{
		Name: "install-rule",
		Type: catalog.Rules,
		Path: sourcePath,
	}

	desc, err := Install(item, prov, repoRoot, MethodSymlink, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	expectedTarget := filepath.Join(tmp, ".testprovider", "rules", "install-rule")
	if desc != expectedTarget {
		t.Errorf("expected target %s, got %s", expectedTarget, desc)
	}

	// Verify symlink was created
	info, err := os.Lstat(expectedTarget)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}
}

func TestInstall_CopyMethod(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)
	t.Setenv("HOME", tmp)

	prov := testProvider("test")

	sourcePath := filepath.Join(repoRoot, "rules", "test", "copy-rule")
	os.MkdirAll(sourcePath, 0755)
	os.WriteFile(filepath.Join(sourcePath, "rule.md"), []byte("# Copy Rule"), 0644)

	item := catalog.ContentItem{
		Name: "copy-rule",
		Type: catalog.Rules,
		Path: sourcePath,
	}

	desc, err := Install(item, prov, repoRoot, MethodCopy, "")
	if err != nil {
		t.Fatalf("Install copy: %v", err)
	}

	expectedTarget := filepath.Join(tmp, ".testprovider", "rules", "copy-rule")
	if desc != expectedTarget {
		t.Errorf("expected target %s, got %s", expectedTarget, desc)
	}

	// Verify it's NOT a symlink
	info, err := os.Lstat(expectedTarget)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("expected regular file, got symlink")
	}
}

func TestInstall_WithBaseDir(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	baseDir := filepath.Join(tmp, "custom-base")
	os.MkdirAll(repoRoot, 0755)

	prov := testProvider("test")

	sourcePath := filepath.Join(repoRoot, "rules", "test", "base-rule")
	os.MkdirAll(sourcePath, 0755)
	os.WriteFile(filepath.Join(sourcePath, "rule.md"), []byte("# Base Rule"), 0644)

	item := catalog.ContentItem{
		Name: "base-rule",
		Type: catalog.Rules,
		Path: sourcePath,
	}

	desc, err := Install(item, prov, repoRoot, MethodSymlink, baseDir)
	if err != nil {
		t.Fatalf("Install with baseDir: %v", err)
	}

	expectedTarget := filepath.Join(baseDir, ".testprovider", "rules", "base-rule")
	if desc != expectedTarget {
		t.Errorf("expected target %s, got %s", expectedTarget, desc)
	}
}

func TestInstall_UnsupportedTypeReturnsError(t *testing.T) {
	t.Parallel()
	prov := testProvider("test")
	item := catalog.ContentItem{
		Name: "cmd",
		Type: catalog.Commands,
		Path: "/repo/commands/test/cmd",
	}

	_, err := Install(item, prov, "/repo", MethodSymlink, "/home/user")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestInstall_AgentsUsesAGENTMD(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)
	t.Setenv("HOME", tmp)

	prov := testProvider("test")

	sourcePath := filepath.Join(repoRoot, "agents", "test", "my-agent")
	os.MkdirAll(sourcePath, 0755)
	os.WriteFile(filepath.Join(sourcePath, "AGENT.md"), []byte("# Agent"), 0644)

	item := catalog.ContentItem{
		Name: "my-agent",
		Type: catalog.Agents,
		Path: sourcePath,
	}

	desc, err := Install(item, prov, repoRoot, MethodSymlink, "")
	if err != nil {
		t.Fatalf("Install agent: %v", err)
	}

	expectedTarget := filepath.Join(tmp, ".testprovider", "agents", "my-agent.md")
	if desc != expectedTarget {
		t.Errorf("expected target %s, got %s", expectedTarget, desc)
	}

	// Verify symlink points to the AGENT.md file, not the directory
	link, err := os.Readlink(expectedTarget)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if filepath.Base(link) != "AGENT.md" {
		t.Errorf("expected symlink to AGENT.md, got %s", link)
	}
}

func TestUninstall_RemovesSymlink(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)
	t.Setenv("HOME", tmp)

	prov := testProvider("test")

	sourcePath := filepath.Join(repoRoot, "rules", "test", "remove-rule")
	os.MkdirAll(sourcePath, 0755)

	item := catalog.ContentItem{
		Name: "remove-rule",
		Type: catalog.Rules,
		Path: sourcePath,
	}

	targetPath := filepath.Join(tmp, ".testprovider", "rules", "remove-rule")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.Symlink(sourcePath, targetPath)

	desc, err := Uninstall(item, prov, repoRoot)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if desc != targetPath {
		t.Errorf("expected desc %s, got %s", targetPath, desc)
	}

	// Verify symlink is gone
	if _, err := os.Lstat(targetPath); !os.IsNotExist(err) {
		t.Error("symlink should be removed")
	}
}

func TestUninstall_RemovesRegularFile(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)
	t.Setenv("HOME", tmp)

	prov := testProvider("test")

	item := catalog.ContentItem{
		Name: "copied-rule",
		Type: catalog.Rules,
		Path: filepath.Join(repoRoot, "rules", "test", "copied-rule"),
	}

	targetPath := filepath.Join(tmp, ".testprovider", "rules", "copied-rule")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.WriteFile(targetPath, []byte("# Copied"), 0644)

	desc, err := Uninstall(item, prov, repoRoot)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if desc != targetPath {
		t.Errorf("expected desc %s, got %s", targetPath, desc)
	}

	if _, err := os.Lstat(targetPath); !os.IsNotExist(err) {
		t.Error("file should be removed")
	}
}

func TestUninstall_RemovesDirectory(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)
	t.Setenv("HOME", tmp)

	prov := testProvider("test")

	item := catalog.ContentItem{
		Name: "dir-rule",
		Type: catalog.Rules,
		Path: filepath.Join(repoRoot, "rules", "test", "dir-rule"),
	}

	targetPath := filepath.Join(tmp, ".testprovider", "rules", "dir-rule")
	os.MkdirAll(targetPath, 0755)
	os.WriteFile(filepath.Join(targetPath, "file.md"), []byte("# Content"), 0644)

	desc, err := Uninstall(item, prov, repoRoot)
	if err != nil {
		t.Fatalf("Uninstall dir: %v", err)
	}
	if desc != targetPath {
		t.Errorf("expected desc %s, got %s", targetPath, desc)
	}

	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Error("directory should be removed")
	}
}

func TestUninstall_NotInstalledReturnsError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	prov := testProvider("test")

	item := catalog.ContentItem{
		Name: "missing-rule",
		Type: catalog.Rules,
		Path: "/repo/rules/test/missing-rule",
	}

	_, err := Uninstall(item, prov, "/repo")
	if err == nil {
		t.Fatal("expected error for not installed item")
	}
}

func TestCheckStatusWithResolver_AgentsType(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	customDir := filepath.Join(tmp, "custom", "agents")
	os.MkdirAll(customDir, 0755)
	os.MkdirAll(repoRoot, 0755)

	prov := testProvider("test")

	item := catalog.ContentItem{
		Name:     "my-agent",
		Type:     catalog.Agents,
		Provider: "test",
		Path:     filepath.Join(repoRoot, "agents", "test", "my-agent"),
	}
	os.MkdirAll(item.Path, 0755)

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test": {Paths: map[string]string{"agents": customDir}},
		},
	}
	resolver := config.NewResolver(cfg, "")

	// Not installed
	status := CheckStatusWithResolver(item, prov, repoRoot, resolver)
	if status != StatusNotInstalled {
		t.Errorf("expected StatusNotInstalled, got %v", status)
	}

	// "Install" at custom path
	targetPath := filepath.Join(customDir, "my-agent.md")
	os.WriteFile(targetPath, []byte("# Agent"), 0644)

	status = CheckStatusWithResolver(item, prov, repoRoot, resolver)
	if status != StatusInstalled {
		t.Errorf("expected StatusInstalled, got %v", status)
	}
}

func TestCheckStatus_MergeTypeNotAvailable(t *testing.T) {
	// Test the fallback case where a merge type is not MCP or Hooks
	t.Parallel()

	// Create a provider that returns JSONMergeSentinel for Loadouts (a type that
	// doesn't have a merge handler)
	prov := provider.Provider{
		Name: "Merge Test",
		Slug: "merge-test",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			if ct == catalog.Loadouts {
				return provider.JSONMergeSentinel
			}
			return ""
		},
	}

	item := catalog.ContentItem{
		Name: "test",
		Type: catalog.Loadouts,
		Path: "/some/path",
	}

	status := CheckStatus(item, prov, "/repo")
	if status != StatusNotAvailable {
		t.Errorf("expected StatusNotAvailable for unsupported merge type, got %v", status)
	}
}

func TestInstallWithRenderTo(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Create a canonical rule content file
	itemDir := filepath.Join(tmp, "content", "rules", "claude-code", "cross-rule")
	os.MkdirAll(itemDir, 0755)
	canonicalContent := "---\ndescription: Test rule\nalwaysApply: true\n---\n# Test Rule\n\nThis is a test rule.\n"
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte(canonicalContent), 0644)

	item := catalog.ContentItem{
		Name:     "cross-rule",
		Type:     catalog.Rules,
		Provider: "claude-code", // source provider
		Path:     itemDir,
	}

	// Target provider is cursor — cross-provider render
	cursorProv := provider.Provider{
		Name: "Cursor",
		Slug: "cursor",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			if ct == catalog.Rules {
				return filepath.Join(homeDir, ".cursor", "rules")
			}
			return ""
		},
	}

	targetDir := filepath.Join(tmp, "install-target")
	os.MkdirAll(targetDir, 0755)

	conv := converter.For(catalog.Rules)
	if conv == nil {
		t.Fatal("no converter registered for rules")
	}

	desc, err := installWithRenderTo(item, cursorProv, conv, targetDir)
	if err != nil {
		t.Fatalf("installWithRenderTo: %v", err)
	}

	// Should have written a file
	if desc == "" {
		t.Fatal("expected non-empty description")
	}

	// Verify the file exists and has content
	data, err := os.ReadFile(desc)
	if err != nil {
		t.Fatalf("reading rendered file: %v", err)
	}
	if len(data) == 0 {
		t.Error("rendered file is empty")
	}
}

func TestInstallWithRenderTo_NoContentFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Empty item directory — no content file to render from
	itemDir := filepath.Join(tmp, "empty-item")
	os.MkdirAll(itemDir, 0755)

	item := catalog.ContentItem{
		Name:     "empty",
		Type:     catalog.Rules,
		Provider: "claude-code",
		Path:     itemDir,
	}

	cursorProv := provider.Provider{Name: "Cursor", Slug: "cursor"}
	conv := converter.For(catalog.Rules)
	if conv == nil {
		t.Fatal("no converter registered for rules")
	}

	_, err := installWithRenderTo(item, cursorProv, conv, filepath.Join(tmp, "target"))
	if err == nil {
		t.Fatal("expected error for empty content item")
	}
}

func TestInstallFromSourceTo(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Create an item with a .source/ directory containing a file
	itemDir := filepath.Join(tmp, "content", "rules", "test-prov", "my-rule")
	sourceDir := filepath.Join(itemDir, ".source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "original.mdc"), []byte("# Original Format"), 0644)
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# Canonical"), 0644)

	item := catalog.ContentItem{
		Name:     "my-rule",
		Type:     catalog.Rules,
		Provider: "test-prov",
		Path:     itemDir,
	}

	targetDir := filepath.Join(tmp, "install-target")
	os.MkdirAll(targetDir, 0755)

	desc, err := installFromSourceTo(item, testProvider("test-prov"), targetDir)
	if err != nil {
		t.Fatalf("installFromSourceTo: %v", err)
	}

	// Should have written the source file, not the canonical file
	if filepath.Base(desc) != "original.mdc" {
		t.Errorf("expected original.mdc, got %s", filepath.Base(desc))
	}

	data, err := os.ReadFile(desc)
	if err != nil {
		t.Fatalf("reading installed file: %v", err)
	}
	if string(data) != "# Original Format" {
		t.Errorf("content = %q, want '# Original Format'", string(data))
	}
}

func TestInstallFromSourceTo_NoSourceFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Item without .source/ directory
	itemDir := filepath.Join(tmp, "content", "rules", "test-prov", "no-source")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# Rule"), 0644)

	item := catalog.ContentItem{
		Name:     "no-source",
		Type:     catalog.Rules,
		Provider: "test-prov",
		Path:     itemDir,
	}

	_, err := installFromSourceTo(item, testProvider("test-prov"), filepath.Join(tmp, "target"))
	if err == nil {
		t.Fatal("expected error when no .source/ directory exists")
	}
}

func TestInstall_CrossProviderRendering(t *testing.T) {
	// Tests the converter dispatch path in Install() — when item.Provider != prov.Slug.
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)
	t.Setenv("HOME", tmp)

	// Create a canonical rule from "claude-code" provider
	itemDir := filepath.Join(repoRoot, "rules", "claude-code", "cross-rule")
	os.MkdirAll(itemDir, 0755)
	canonicalContent := "---\ndescription: Cross-provider test\nalwaysApply: true\n---\n# Cross Rule\n\nTest content.\n"
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte(canonicalContent), 0644)

	item := catalog.ContentItem{
		Name:     "cross-rule",
		Type:     catalog.Rules,
		Provider: "claude-code",
		Path:     itemDir,
	}

	// Install to cursor (different provider) — should trigger converter rendering
	cursorProv := provider.Provider{
		Name: "Cursor",
		Slug: "cursor",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			if ct == catalog.Rules {
				return filepath.Join(homeDir, ".cursor", "rules")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
	}

	desc, err := Install(item, cursorProv, repoRoot, MethodSymlink, "")
	if err != nil {
		t.Fatalf("Install cross-provider: %v", err)
	}

	// Verify the file was written
	data, err := os.ReadFile(desc)
	if err != nil {
		t.Fatalf("reading installed file: %v", err)
	}
	if len(data) == 0 {
		t.Error("installed file is empty")
	}
}

func TestInstall_SameProviderWithSource(t *testing.T) {
	// Tests the source file lossless roundtrip path in Install().
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)
	t.Setenv("HOME", tmp)

	// Create a canonical rule with a .source/ directory
	itemDir := filepath.Join(repoRoot, "rules", "cursor", "sourced-rule")
	sourceDir := filepath.Join(itemDir, ".source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "original.mdc"), []byte("# Original Cursor Format"), 0644)
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("---\ndescription: Test\nalwaysApply: true\n---\n# Canonical\n"), 0644)

	item := catalog.ContentItem{
		Name:     "sourced-rule",
		Type:     catalog.Rules,
		Provider: "cursor", // same as target provider
		Path:     itemDir,
	}

	cursorProv := provider.Provider{
		Name: "Cursor",
		Slug: "cursor",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			if ct == catalog.Rules {
				return filepath.Join(homeDir, ".cursor", "rules")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
	}

	desc, err := Install(item, cursorProv, repoRoot, MethodSymlink, "")
	if err != nil {
		t.Fatalf("Install with source: %v", err)
	}

	// Should have installed the original source file
	data, err := os.ReadFile(desc)
	if err != nil {
		t.Fatalf("reading installed file: %v", err)
	}
	if string(data) != "# Original Cursor Format" {
		t.Errorf("expected original content, got %q", string(data))
	}
}

func TestInstallWithResolver_SameProviderWithSource(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	customDir := filepath.Join(tmp, "custom", "rules")
	os.MkdirAll(customDir, 0755)
	os.MkdirAll(repoRoot, 0755)

	// Create item with .source/ directory
	itemDir := filepath.Join(repoRoot, "rules", "cursor", "source-rule")
	sourceDir := filepath.Join(itemDir, ".source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "original.mdc"), []byte("# Source Content"), 0644)
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("---\ndescription: test\nalwaysApply: true\n---\n# Canonical\n"), 0644)

	item := catalog.ContentItem{
		Name:     "source-rule",
		Type:     catalog.Rules,
		Provider: "cursor",
		Path:     itemDir,
	}

	cursorProv := provider.Provider{
		Name: "Cursor",
		Slug: "cursor",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			if ct == catalog.Rules {
				return filepath.Join(homeDir, ".cursor", "rules")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
	}

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"cursor": {Paths: map[string]string{"rules": customDir}},
		},
	}
	resolver := config.NewResolver(cfg, "")

	desc, err := InstallWithResolver(item, cursorProv, repoRoot, MethodSymlink, resolver)
	if err != nil {
		t.Fatalf("InstallWithResolver: %v", err)
	}

	data, err := os.ReadFile(desc)
	if err != nil {
		t.Fatalf("reading installed file: %v", err)
	}
	if string(data) != "# Source Content" {
		t.Errorf("expected source content, got %q", string(data))
	}
}

func TestInstallWithResolver_CrossProviderRendering(t *testing.T) {
	// Tests the converter dispatch path via InstallWithResolver.
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	customDir := filepath.Join(tmp, "custom", "rules")
	os.MkdirAll(customDir, 0755)
	os.MkdirAll(repoRoot, 0755)

	// Create a canonical rule from "claude-code"
	itemDir := filepath.Join(repoRoot, "rules", "claude-code", "resolver-cross")
	os.MkdirAll(itemDir, 0755)
	canonicalContent := "---\ndescription: Resolver cross test\nalwaysApply: true\n---\n# Resolver Cross\n\nContent.\n"
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte(canonicalContent), 0644)

	item := catalog.ContentItem{
		Name:     "resolver-cross",
		Type:     catalog.Rules,
		Provider: "claude-code",
		Path:     itemDir,
	}

	cursorProv := provider.Provider{
		Name: "Cursor",
		Slug: "cursor",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			if ct == catalog.Rules {
				return filepath.Join(homeDir, ".cursor", "rules")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
	}

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"cursor": {Paths: map[string]string{"rules": customDir}},
		},
	}
	resolver := config.NewResolver(cfg, "")

	desc, err := InstallWithResolver(item, cursorProv, repoRoot, MethodSymlink, resolver)
	if err != nil {
		t.Fatalf("InstallWithResolver cross-provider: %v", err)
	}

	data, err := os.ReadFile(desc)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if len(data) == 0 {
		t.Error("empty file")
	}
}

func TestInstall_MergeTypeDispatch(t *testing.T) {
	// Verify Install dispatches to MCP/Hooks handlers for merge types.
	t.Parallel()

	prov := testProvider("test")

	item := catalog.ContentItem{
		Name: "test",
		Type: catalog.Hooks,
		Path: "/nonexistent/path",
	}

	_, err := Install(item, prov, "/repo", MethodSymlink, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !contains(err.Error(), "parsing hook file") {
		t.Errorf("expected hook parsing error, got: %v", err)
	}
}

func TestInstallWithResolver_UnsupportedType(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)

	prov := provider.Provider{
		Name: "Empty",
		Slug: "empty",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			return "" // doesn't support anything
		},
	}

	item := catalog.ContentItem{
		Name: "cmd",
		Type: catalog.Commands,
		Path: filepath.Join(repoRoot, "commands", "cmd"),
	}

	resolver := config.NewResolver(&config.Config{}, "")
	_, err := InstallWithResolver(item, prov, repoRoot, MethodSymlink, resolver)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestInstallWithResolver_ProjectScopeSentinel(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)

	prov := testProviderProjectScope("projprov")

	item := catalog.ContentItem{
		Name: "rule",
		Type: catalog.Rules,
		Path: filepath.Join(repoRoot, "rules", "projprov", "rule"),
	}

	resolver := config.NewResolver(&config.Config{}, "")
	_, err := InstallWithResolver(item, prov, repoRoot, MethodSymlink, resolver)
	if err == nil {
		t.Fatal("expected error for project-scoped type")
	}
}

func TestInstallWithResolver_AgentsType(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	customDir := filepath.Join(tmp, "custom", "agents")
	os.MkdirAll(customDir, 0755)

	prov := testProvider("test")

	sourcePath := filepath.Join(repoRoot, "agents", "test", "my-agent")
	os.MkdirAll(sourcePath, 0755)
	os.WriteFile(filepath.Join(sourcePath, "AGENT.md"), []byte("# Agent"), 0644)

	item := catalog.ContentItem{
		Name:     "my-agent",
		Type:     catalog.Agents,
		Provider: "test",
		Path:     sourcePath,
	}

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test": {Paths: map[string]string{"agents": customDir}},
		},
	}
	resolver := config.NewResolver(cfg, "")

	desc, err := InstallWithResolver(item, prov, repoRoot, MethodSymlink, resolver)
	if err != nil {
		t.Fatalf("InstallWithResolver agents: %v", err)
	}

	expected := filepath.Join(customDir, "my-agent.md")
	if desc != expected {
		t.Errorf("expected %s, got %s", expected, desc)
	}
}

func TestInstall_MergeTypeMCPDispatch(t *testing.T) {
	// Not parallel — mutates package-level mcpConfigPath
	tmpDir := t.TempDir()

	prov := testProvider("test")

	// Create MCP item
	itemDir := filepath.Join(tmpDir, "mcp-dispatch")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{"command":"node","args":["s.js"]}`), 0644)

	configFile := filepath.Join(tmpDir, ".claude.json")
	os.WriteFile(configFile, []byte("{}"), 0644)

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	item := catalog.ContentItem{
		Name: "mcp-dispatch",
		Type: catalog.MCP,
		Path: itemDir,
	}

	desc, err := Install(item, prov, tmpDir, MethodSymlink, "")
	if err != nil {
		t.Fatalf("Install MCP: %v", err)
	}
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestUninstall_MergeTypeDispatch(t *testing.T) {
	t.Parallel()

	prov := testProvider("test")

	item := catalog.ContentItem{
		Name: "test",
		Type: catalog.Hooks,
		Path: "/nonexistent/path", // Will fail at parseHookFile
	}

	_, err := Uninstall(item, prov, "/repo")
	if err == nil {
		t.Fatal("expected error")
	}
	if !contains(err.Error(), "parsing hook file") {
		t.Errorf("expected hook parsing error, got: %v", err)
	}
}

// contains is a helper for string containment checks.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestStatusString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status Status
		want   string
	}{
		{"installed renders with text label", StatusInstalled, "[ok]"},
		{"not installed renders with text label", StatusNotInstalled, "[--]"},
		{"not available renders with text label", StatusNotAvailable, "[-]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.status.String()
			if got != tt.want {
				t.Errorf("Status.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
