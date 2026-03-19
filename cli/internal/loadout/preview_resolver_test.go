package loadout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func stubProviderForPreview(slug string) provider.Provider {
	return provider.Provider{
		Name:      "Test Provider",
		Slug:      slug,
		ConfigDir: ".provider",
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

func TestPreview_ResolverPerTypePath(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	homeDir := t.TempDir()

	// Set up .syllago dir (Preview reads installed.json from here)
	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	prov := stubProviderForPreview("test-prov")

	// Create a rule source in the repo
	ruleDir := filepath.Join(repoRoot, "content", "rules", "test-prov", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Rule"), 0644)

	refs := []ResolvedRef{
		{
			Type: catalog.Rules,
			Name: "my-rule",
			Item: catalog.ContentItem{
				Name:     "my-rule",
				Type:     catalog.Rules,
				Provider: "test-prov",
				Path:     ruleDir,
			},
		},
	}

	customRulesDir := filepath.Join(homeDir, "custom-rules")

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {Paths: map[string]string{"rules": customRulesDir}},
		},
	}
	resolver := config.NewResolver(cfg, "")

	actions, err := Preview(refs, prov, repoRoot, homeDir, resolver)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	action := actions[0]
	if action.Action != "create-symlink" {
		t.Errorf("expected create-symlink, got %q", action.Action)
	}
	// Target should be under the custom dir, not the default provider path
	if filepath.Dir(action.Detail) != customRulesDir {
		t.Errorf("expected target under %q, got %q", customRulesDir, action.Detail)
	}
}

func TestPreview_ResolverBaseDir(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	customBase := t.TempDir()

	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	prov := stubProviderForPreview("test-prov")

	ruleDir := filepath.Join(repoRoot, "content", "rules", "test-prov", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Rule"), 0644)

	refs := []ResolvedRef{
		{
			Type: catalog.Rules,
			Name: "my-rule",
			Item: catalog.ContentItem{
				Name:     "my-rule",
				Type:     catalog.Rules,
				Provider: "test-prov",
				Path:     ruleDir,
			},
		},
	}

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {BaseDir: customBase},
		},
	}
	resolver := config.NewResolver(cfg, "")

	actions, err := Preview(refs, prov, repoRoot, homeDir, resolver)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	// Target should be under customBase/.provider/rules/, not homeDir
	expectedDir := filepath.Join(customBase, ".provider", "rules")
	if filepath.Dir(actions[0].Detail) != expectedDir {
		t.Errorf("expected target under %q, got %q", expectedDir, actions[0].Detail)
	}
}

func TestPreview_NilResolverUsesDefault(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	homeDir := t.TempDir()

	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	prov := stubProviderForPreview("test-prov")

	ruleDir := filepath.Join(repoRoot, "content", "rules", "test-prov", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Rule"), 0644)

	refs := []ResolvedRef{
		{
			Type: catalog.Rules,
			Name: "my-rule",
			Item: catalog.ContentItem{
				Name:     "my-rule",
				Type:     catalog.Rules,
				Provider: "test-prov",
				Path:     ruleDir,
			},
		},
	}

	actions, err := Preview(refs, prov, repoRoot, homeDir, nil)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	// Target should be under homeDir/.provider/rules/
	expectedDir := filepath.Join(homeDir, ".provider", "rules")
	if filepath.Dir(actions[0].Detail) != expectedDir {
		t.Errorf("expected target under %q, got %q", expectedDir, actions[0].Detail)
	}
}

func TestPreview_ResolverSkipsExistingSymlink(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	homeDir := t.TempDir()

	os.MkdirAll(filepath.Join(repoRoot, ".syllago"), 0755)

	prov := stubProviderForPreview("test-prov")

	ruleDir := filepath.Join(repoRoot, "content", "rules", "test-prov", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Rule"), 0644)

	customRulesDir := filepath.Join(homeDir, "custom-rules")
	os.MkdirAll(customRulesDir, 0755)

	// Pre-create a symlink at the target pointing to the source
	targetPath := filepath.Join(customRulesDir, "my-rule")
	if err := os.Symlink(ruleDir, targetPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	refs := []ResolvedRef{
		{
			Type: catalog.Rules,
			Name: "my-rule",
			Item: catalog.ContentItem{
				Name:     "my-rule",
				Type:     catalog.Rules,
				Provider: "test-prov",
				Path:     ruleDir,
			},
		},
	}

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {Paths: map[string]string{"rules": customRulesDir}},
		},
	}
	resolver := config.NewResolver(cfg, "")

	actions, err := Preview(refs, prov, repoRoot, homeDir, resolver)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "skip-exists" {
		t.Errorf("expected skip-exists for existing symlink, got %q", actions[0].Action)
	}
}
