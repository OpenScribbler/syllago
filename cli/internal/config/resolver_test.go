package config

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// stubProvider returns a minimal provider for testing.
func stubProvider(slug string) provider.Provider {
	return provider.Provider{
		Slug: slug,
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			return homeDir + "/.provider/" + string(ct)
		},
		DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
			return []string{projectRoot + "/.provider/" + string(ct)}
		},
	}
}

func TestResolver_DefaultPassthrough(t *testing.T) {
	t.Parallel()
	r := NewResolver(&Config{}, "")
	prov := stubProvider("test")

	got := r.InstallDir(prov, catalog.Skills, "/home/user")
	want := "/home/user/.provider/skills"
	if got != want {
		t.Errorf("InstallDir = %q, want %q", got, want)
	}

	paths := r.DiscoveryPaths(prov, catalog.Skills, "/project")
	if len(paths) != 1 || paths[0] != "/project/.provider/skills" {
		t.Errorf("DiscoveryPaths = %v, want [/project/.provider/skills]", paths)
	}
}

func TestResolver_NilResolver(t *testing.T) {
	t.Parallel()
	var r *PathResolver
	prov := stubProvider("test")

	got := r.InstallDir(prov, catalog.Skills, "/home/user")
	if got != "/home/user/.provider/skills" {
		t.Errorf("nil resolver InstallDir = %q, want default", got)
	}

	paths := r.DiscoveryPaths(prov, catalog.Skills, "/project")
	if len(paths) != 1 || paths[0] != "/project/.provider/skills" {
		t.Errorf("nil resolver DiscoveryPaths = %v, want default", paths)
	}
}

func TestResolver_PerTypePath(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"test": {Paths: map[string]string{"skills": "/custom/skills"}},
		},
	}
	r := NewResolver(cfg, "")
	prov := stubProvider("test")

	// Per-type path bypasses provider logic
	got := r.InstallDir(prov, catalog.Skills, "/home/user")
	if got != "/custom/skills" {
		t.Errorf("InstallDir with per-type = %q, want /custom/skills", got)
	}

	paths := r.DiscoveryPaths(prov, catalog.Skills, "/project")
	if len(paths) != 1 || paths[0] != "/custom/skills" {
		t.Errorf("DiscoveryPaths with per-type = %v, want [/custom/skills]", paths)
	}

	// Other types still use default
	got = r.InstallDir(prov, catalog.Hooks, "/home/user")
	if got != "/home/user/.provider/hooks" {
		t.Errorf("InstallDir for hooks = %q, want default", got)
	}
}

func TestResolver_BaseDir(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"test": {BaseDir: "/config/base"},
		},
	}
	r := NewResolver(cfg, "")
	prov := stubProvider("test")

	got := r.InstallDir(prov, catalog.Skills, "/home/user")
	if got != "/config/base/.provider/skills" {
		t.Errorf("InstallDir with baseDir = %q, want /config/base/.provider/skills", got)
	}

	paths := r.DiscoveryPaths(prov, catalog.Skills, "/project")
	if len(paths) != 1 || paths[0] != "/config/base/.provider/skills" {
		t.Errorf("DiscoveryPaths with baseDir = %v, want [/config/base/.provider/skills]", paths)
	}
}

func TestResolver_CLIBaseDirOverridesConfig(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"test": {BaseDir: "/config/base"},
		},
	}
	r := NewResolver(cfg, "/cli/override")
	prov := stubProvider("test")

	got := r.InstallDir(prov, catalog.Skills, "/home/user")
	if got != "/cli/override/.provider/skills" {
		t.Errorf("InstallDir CLI override = %q, want /cli/override/.provider/skills", got)
	}
}

func TestResolver_PerTypeOverridesBaseDir(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"test": {
				BaseDir: "/config/base",
				Paths:   map[string]string{"skills": "/direct/skills"},
			},
		},
	}
	r := NewResolver(cfg, "")
	prov := stubProvider("test")

	// Per-type wins over baseDir
	got := r.InstallDir(prov, catalog.Skills, "/home/user")
	if got != "/direct/skills" {
		t.Errorf("per-type should win over baseDir, got %q", got)
	}

	// baseDir still applies to other types
	got = r.InstallDir(prov, catalog.Hooks, "/home/user")
	if got != "/config/base/.provider/hooks" {
		t.Errorf("hooks should use baseDir, got %q", got)
	}
}

func TestResolver_PerTypeOverridesCLIBaseDir(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"test": {Paths: map[string]string{"skills": "/direct/skills"}},
		},
	}
	r := NewResolver(cfg, "/cli/override")
	prov := stubProvider("test")

	// Per-type wins over CLI --base-dir too
	got := r.InstallDir(prov, catalog.Skills, "/home/user")
	if got != "/direct/skills" {
		t.Errorf("per-type should win over CLI baseDir, got %q", got)
	}
}

func TestResolver_ExpandPaths(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"test": {
				BaseDir: "/absolute/path",
				Paths:   map[string]string{"skills": "/absolute/skills"},
			},
		},
	}
	r := NewResolver(cfg, "/cli/path")
	if err := r.ExpandPaths(); err != nil {
		t.Fatalf("ExpandPaths: %v", err)
	}
	// Absolute paths should pass through unchanged
	if r.CLIBaseDir != "/cli/path" {
		t.Errorf("CLIBaseDir = %q, want /cli/path", r.CLIBaseDir)
	}
	if r.ProviderPaths["test"].BaseDir != "/absolute/path" {
		t.Errorf("BaseDir = %q, want /absolute/path", r.ProviderPaths["test"].BaseDir)
	}
}

func TestResolver_ExpandPathsNil(t *testing.T) {
	t.Parallel()
	var r *PathResolver
	if err := r.ExpandPaths(); err != nil {
		t.Errorf("ExpandPaths on nil should not error: %v", err)
	}
}

func TestResolver_PathCanonicalization(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"test": {Paths: map[string]string{"skills": "/custom/./skills/../skills/"}},
		},
	}
	r := NewResolver(cfg, "")
	prov := stubProvider("test")

	got := r.InstallDir(prov, catalog.Skills, "/home/user")
	if got != "/custom/skills" {
		t.Errorf("path should be cleaned, got %q", got)
	}
}

func TestResolver_HasPerTypePath_NilReceiver(t *testing.T) {
	t.Parallel()
	var r *PathResolver
	// Should not panic on nil receiver
	if r.HasPerTypePath("test", catalog.Skills) {
		t.Error("nil resolver HasPerTypePath should return false")
	}
}

func TestResolver_BaseDir_Priority(t *testing.T) {
	t.Parallel()

	t.Run("nil resolver", func(t *testing.T) {
		var r *PathResolver
		if got := r.BaseDir("test"); got != "" {
			t.Errorf("nil resolver BaseDir = %q, want empty", got)
		}
	})

	t.Run("CLI overrides config", func(t *testing.T) {
		cfg := &Config{
			ProviderPaths: map[string]ProviderPathConfig{
				"test": {BaseDir: "/config/base"},
			},
		}
		r := NewResolver(cfg, "/cli/override")
		if got := r.BaseDir("test"); got != "/cli/override" {
			t.Errorf("BaseDir = %q, want /cli/override", got)
		}
	})

	t.Run("config fallback", func(t *testing.T) {
		cfg := &Config{
			ProviderPaths: map[string]ProviderPathConfig{
				"test": {BaseDir: "/config/base"},
			},
		}
		r := NewResolver(cfg, "")
		if got := r.BaseDir("test"); got != "/config/base" {
			t.Errorf("BaseDir = %q, want /config/base", got)
		}
	})

	t.Run("empty when no overrides", func(t *testing.T) {
		r := NewResolver(&Config{}, "")
		if got := r.BaseDir("test"); got != "" {
			t.Errorf("BaseDir = %q, want empty", got)
		}
	})
}

func TestResolver_UnknownProvider(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		ProviderPaths: map[string]ProviderPathConfig{
			"other": {BaseDir: "/other/base"},
		},
	}
	r := NewResolver(cfg, "")
	prov := stubProvider("test")

	// "test" provider has no overrides — should fall through to default
	got := r.InstallDir(prov, catalog.Skills, "/home/user")
	if got != "/home/user/.provider/skills" {
		t.Errorf("unknown provider should use default, got %q", got)
	}
}
