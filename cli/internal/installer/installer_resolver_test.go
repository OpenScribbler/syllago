package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func stubProviderForInstaller(slug string) provider.Provider {
	return provider.Provider{
		Name: "Test Provider",
		Slug: slug,
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			switch ct {
			case catalog.Rules:
				return filepath.Join(homeDir, ".provider", "rules")
			case catalog.Skills:
				return filepath.Join(homeDir, ".provider", "skills")
			case catalog.Hooks:
				return "__json_merge__"
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			switch ct {
			case catalog.Rules, catalog.Skills, catalog.Hooks:
				return true
			}
			return false
		},
	}
}

func TestCheckStatusWithResolver_PerTypePath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	customDir := filepath.Join(tmp, "custom", "rules")
	os.MkdirAll(customDir, 0755)
	os.MkdirAll(repoRoot, 0755)

	prov := stubProviderForInstaller("test-prov")

	item := catalog.ContentItem{
		Name:     "my-rule",
		Type:     catalog.Rules,
		Provider: "test-prov",
		Path:     filepath.Join(repoRoot, "content", "rules", "test-prov", "my-rule"),
	}

	// Create the source directory
	os.MkdirAll(item.Path, 0755)
	os.WriteFile(filepath.Join(item.Path, "rule.md"), []byte("# Rule"), 0644)

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {Paths: map[string]string{"rules": customDir}},
		},
	}
	resolver := config.NewResolver(cfg, "")

	// Not installed yet — no file at custom path
	status := CheckStatusWithResolver(item, prov, repoRoot, resolver)
	if status != StatusNotInstalled {
		t.Errorf("expected StatusNotInstalled, got %v", status)
	}

	// "Install" by creating a file at the custom path
	targetPath := filepath.Join(customDir, "my-rule")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.WriteFile(targetPath, []byte("# Installed"), 0644)

	status = CheckStatusWithResolver(item, prov, repoRoot, resolver)
	if status != StatusInstalled {
		t.Errorf("expected StatusInstalled after file creation, got %v", status)
	}
}

func TestCheckStatusWithResolver_BaseDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	customBase := filepath.Join(tmp, "custom-base")
	os.MkdirAll(repoRoot, 0755)

	prov := stubProviderForInstaller("test-prov")

	item := catalog.ContentItem{
		Name:     "my-rule",
		Type:     catalog.Rules,
		Provider: "test-prov",
		Path:     filepath.Join(repoRoot, "content", "rules", "test-prov", "my-rule"),
	}
	os.MkdirAll(item.Path, 0755)

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {BaseDir: customBase},
		},
	}
	resolver := config.NewResolver(cfg, "")

	// Not installed — custom base doesn't have the file
	status := CheckStatusWithResolver(item, prov, repoRoot, resolver)
	if status != StatusNotInstalled {
		t.Errorf("expected StatusNotInstalled, got %v", status)
	}

	// "Install" at the custom base (mirrors provider structure)
	targetDir := filepath.Join(customBase, ".provider", "rules")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "my-rule"), []byte("# Installed"), 0644)

	status = CheckStatusWithResolver(item, prov, repoRoot, resolver)
	if status != StatusInstalled {
		t.Errorf("expected StatusInstalled with baseDir, got %v", status)
	}
}

func TestCheckStatusWithResolver_CLIOverridesConfig(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)

	prov := stubProviderForInstaller("test-prov")

	item := catalog.ContentItem{
		Name:     "my-rule",
		Type:     catalog.Rules,
		Provider: "test-prov",
		Path:     filepath.Join(repoRoot, "content", "rules", "test-prov", "my-rule"),
	}
	os.MkdirAll(item.Path, 0755)

	// File at config base
	configBase := filepath.Join(tmp, "config-base")
	os.MkdirAll(filepath.Join(configBase, ".provider", "rules"), 0755)
	os.WriteFile(filepath.Join(configBase, ".provider", "rules", "my-rule"), []byte("# Config"), 0644)

	// CLI base is empty
	cliBase := filepath.Join(tmp, "cli-base")

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {BaseDir: configBase},
		},
	}
	resolver := config.NewResolver(cfg, cliBase)

	// CLI base wins — file doesn't exist there, so not installed
	status := CheckStatusWithResolver(item, prov, repoRoot, resolver)
	if status != StatusNotInstalled {
		t.Errorf("expected StatusNotInstalled (CLI base empty), got %v", status)
	}
}

func TestCheckStatusWithResolver_SymlinkDetection(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	customDir := filepath.Join(tmp, "custom", "rules")
	os.MkdirAll(customDir, 0755)
	os.MkdirAll(repoRoot, 0755)

	prov := stubProviderForInstaller("test-prov")

	sourcePath := filepath.Join(repoRoot, "content", "rules", "test-prov", "my-rule")
	os.MkdirAll(sourcePath, 0755)
	os.WriteFile(filepath.Join(sourcePath, "rule.md"), []byte("# Rule"), 0644)

	item := catalog.ContentItem{
		Name:     "my-rule",
		Type:     catalog.Rules,
		Provider: "test-prov",
		Path:     sourcePath,
	}

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {Paths: map[string]string{"rules": customDir}},
		},
	}
	resolver := config.NewResolver(cfg, "")

	// Create a symlink at the custom path pointing into the repo
	targetPath := filepath.Join(customDir, "my-rule")
	if err := os.Symlink(sourcePath, targetPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	status := CheckStatusWithResolver(item, prov, repoRoot, resolver)
	if status != StatusInstalled {
		t.Errorf("expected StatusInstalled for symlink to repo, got %v", status)
	}
}

func TestCheckStatusWithResolver_MergeTypeBypassesResolver(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	prov := stubProviderForInstaller("test-prov")

	// Create a valid hook file so checkHookStatus can parse it
	hookDir := filepath.Join(repoRoot, "content", "hooks", "test-prov", "my-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"event":"PostToolUse","matcher":".*","hooks":[{"type":"command","command":"echo test"}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	item := catalog.ContentItem{
		Name:     "my-hook",
		Type:     catalog.Hooks,
		Provider: "test-prov",
		Path:     hookDir,
	}

	// Resolver has a per-type path for hooks — should be irrelevant since
	// hooks use JSON merge, not filesystem paths.
	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {Paths: map[string]string{"hooks": "/should/not/matter"}},
		},
	}
	resolver := config.NewResolver(cfg, "/also/irrelevant")

	// Should dispatch to checkHookStatus (merge path), not the resolver path.
	// Hook exists on disk but not in installed.json, so status is NotInstalled.
	status := CheckStatusWithResolver(item, prov, repoRoot, resolver)
	if status != StatusNotInstalled {
		t.Errorf("expected StatusNotInstalled for hooks (merge type), got %v", status)
	}
}

func TestCheckStatusWithResolver_NilResolver(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0755)

	prov := stubProviderForInstaller("test-prov")

	item := catalog.ContentItem{
		Name:     "my-rule",
		Type:     catalog.Rules,
		Provider: "test-prov",
		Path:     filepath.Join(repoRoot, "content", "rules", "test-prov", "my-rule"),
	}
	os.MkdirAll(item.Path, 0755)

	// nil resolver should use default home-based paths (not panic)
	status := CheckStatusWithResolver(item, prov, repoRoot, nil)
	if status != StatusNotInstalled {
		t.Errorf("expected StatusNotInstalled with nil resolver, got %v", status)
	}
}
