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

func TestCheckPrivacyGate_PrivateToPublic_Blocked(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "community/rules", URL: "https://github.com/community/rules", Visibility: "public", VisibilityCheckedAt: &now},
		},
	}
	os.MkdirAll(filepath.Join(root, ".syllago"), 0755)
	config.Save(root, cfg)

	item := catalog.ContentItem{
		Name: "secret-rule",
		Type: catalog.Rules,
		Meta: &metadata.Meta{
			SourceRegistry:   "acme/internal",
			SourceVisibility: "private",
		},
	}

	err := CheckPrivacyGate(item, "community/rules", root)
	if err == nil {
		t.Fatal("expected G1 gate to block private->public, got nil")
	}
	if !strings.Contains(err.Error(), "cannot publish") {
		t.Errorf("error should contain 'cannot publish', got: %s", err)
	}
}

func TestCheckPrivacyGate_PrivateToPrivate_Allowed(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "acme/other", URL: "https://github.com/acme/other", Visibility: "private", VisibilityCheckedAt: &now},
		},
	}
	os.MkdirAll(filepath.Join(root, ".syllago"), 0755)
	config.Save(root, cfg)

	item := catalog.ContentItem{
		Name: "secret-rule",
		Type: catalog.Rules,
		Meta: &metadata.Meta{
			SourceRegistry:   "acme/internal",
			SourceVisibility: "private",
		},
	}

	err := CheckPrivacyGate(item, "acme/other", root)
	if err != nil {
		t.Fatalf("private->private should be allowed, got: %s", err)
	}
}

func TestCheckPrivacyGate_PublicContent_Allowed(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "community/rules", URL: "https://github.com/community/rules", Visibility: "public", VisibilityCheckedAt: &now},
		},
	}
	os.MkdirAll(filepath.Join(root, ".syllago"), 0755)
	config.Save(root, cfg)

	item := catalog.ContentItem{
		Name: "open-rule",
		Type: catalog.Rules,
		Meta: &metadata.Meta{
			SourceRegistry:   "opensource/standards",
			SourceVisibility: "public",
		},
	}

	err := CheckPrivacyGate(item, "community/rules", root)
	if err != nil {
		t.Fatalf("public->public should be allowed, got: %s", err)
	}
}

func TestCheckPrivacyGate_UntaintedContent_Allowed(t *testing.T) {
	root := t.TempDir()

	item := catalog.ContentItem{
		Name: "local-rule",
		Type: catalog.Rules,
		Meta: &metadata.Meta{SourceType: "provider"},
	}

	err := CheckPrivacyGate(item, "any-registry", root)
	if err != nil {
		t.Fatalf("untainted should be allowed, got: %s", err)
	}
}

func TestCheckPrivacyGate_NilMeta_Allowed(t *testing.T) {
	root := t.TempDir()
	item := catalog.ContentItem{Name: "no-meta", Type: catalog.Rules}

	err := CheckPrivacyGate(item, "any-registry", root)
	if err != nil {
		t.Fatalf("nil meta should be allowed, got: %s", err)
	}
}

func TestCheckSharePrivacyGate_PrivateToPublicRepo_Blocked(t *testing.T) {
	orig := registry.OverrideProbeForTest
	registry.OverrideProbeForTest = func(url string) (string, error) {
		return registry.VisibilityPublic, nil
	}
	t.Cleanup(func() { registry.OverrideProbeForTest = orig })

	root := t.TempDir()
	cmd := exec.Command("git", "init", root)
	cmd.Run()
	cmd = exec.Command("git", "-C", root, "remote", "add", "origin", "https://github.com/public/repo")
	cmd.Run()

	item := catalog.ContentItem{
		Name: "secret-rule",
		Type: catalog.Rules,
		Meta: &metadata.Meta{
			SourceRegistry:   "acme/internal",
			SourceVisibility: "private",
		},
	}

	err := CheckSharePrivacyGate(item, root)
	if err == nil {
		t.Fatal("expected G2 gate to block private->public repo, got nil")
	}
	if !strings.Contains(err.Error(), "cannot share") {
		t.Errorf("error should contain 'cannot share', got: %s", err)
	}
}

func TestCheckSharePrivacyGate_PrivateToPrivateRepo_Allowed(t *testing.T) {
	orig := registry.OverrideProbeForTest
	registry.OverrideProbeForTest = func(url string) (string, error) {
		return registry.VisibilityPrivate, nil
	}
	t.Cleanup(func() { registry.OverrideProbeForTest = orig })

	root := t.TempDir()
	cmd := exec.Command("git", "init", root)
	cmd.Run()
	cmd = exec.Command("git", "-C", root, "remote", "add", "origin", "https://github.com/private/repo")
	cmd.Run()

	item := catalog.ContentItem{
		Name: "secret-rule",
		Type: catalog.Rules,
		Meta: &metadata.Meta{
			SourceRegistry:   "acme/internal",
			SourceVisibility: "private",
		},
	}

	err := CheckSharePrivacyGate(item, root)
	if err != nil {
		t.Fatalf("private->private repo should be allowed, got: %s", err)
	}
}

func TestCheckSharePrivacyGate_PublicContent_Allowed(t *testing.T) {
	item := catalog.ContentItem{
		Name: "open-rule",
		Type: catalog.Rules,
		Meta: &metadata.Meta{
			SourceRegistry:   "community/rules",
			SourceVisibility: "public",
		},
	}

	err := CheckSharePrivacyGate(item, t.TempDir())
	if err != nil {
		t.Fatalf("public content should be allowed, got: %s", err)
	}
}

func TestCheckLoadoutItemTaint_PrivateItemToPublicRegistry_Blocked(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "community/loadouts", URL: "https://github.com/community/loadouts", Visibility: "public", VisibilityCheckedAt: &now},
		},
	}
	os.MkdirAll(filepath.Join(root, ".syllago"), 0755)
	config.Save(root, cfg)

	// Create a loadout manifest referencing a rule
	loadoutDir := t.TempDir()
	os.MkdirAll(loadoutDir, 0755)
	os.WriteFile(filepath.Join(loadoutDir, "loadout.yaml"), []byte(`kind: loadout
version: 1
provider: claude-code
name: test-loadout
description: test
rules:
  - name: private-rule
`), 0644)

	// Create library item with private taint
	globalDir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	itemDir := filepath.Join(globalDir, "rules", "claude-code", "private-rule")
	os.MkdirAll(itemDir, 0755)
	meta := &metadata.Meta{
		ID:               "test-id",
		Name:             "private-rule",
		SourceRegistry:   "acme/internal",
		SourceVisibility: "private",
	}
	metadata.Save(itemDir, meta)

	item := catalog.ContentItem{
		Name: "test-loadout",
		Type: catalog.Loadouts,
		Path: loadoutDir,
	}

	err := checkLoadoutItemTaint(item, "community/loadouts", root)
	if err == nil {
		t.Fatal("expected G4 to block loadout with private item targeting public registry")
	}
	if !strings.Contains(err.Error(), "cannot publish loadout") {
		t.Errorf("error should mention 'cannot publish loadout', got: %s", err)
	}
}

func TestCheckLoadoutItemTaint_PrivateTarget_Allowed(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "acme/loadouts", URL: "https://github.com/acme/loadouts", Visibility: "private", VisibilityCheckedAt: &now},
		},
	}
	os.MkdirAll(filepath.Join(root, ".syllago"), 0755)
	config.Save(root, cfg)

	loadoutDir := t.TempDir()
	os.WriteFile(filepath.Join(loadoutDir, "loadout.yaml"), []byte(`kind: loadout
version: 1
provider: claude-code
name: test-loadout
description: test
rules:
  - name: private-rule
`), 0644)

	item := catalog.ContentItem{
		Name: "test-loadout",
		Type: catalog.Loadouts,
		Path: loadoutDir,
	}

	err := checkLoadoutItemTaint(item, "acme/loadouts", root)
	if err != nil {
		t.Fatalf("private target should be allowed, got: %s", err)
	}
}

func TestCheckLoadoutItemTaint_BadManifest_Skips(t *testing.T) {
	root := t.TempDir()
	loadoutDir := t.TempDir()
	// No loadout.yaml exists — should skip G4 gracefully

	item := catalog.ContentItem{
		Name: "missing-manifest",
		Type: catalog.Loadouts,
		Path: loadoutDir,
	}

	err := checkLoadoutItemTaint(item, "any-registry", root)
	if err != nil {
		t.Fatalf("bad manifest should be skipped, got: %s", err)
	}
}
