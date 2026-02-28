package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/metadata"
)

func TestCreateSkill(t *testing.T) {
	root := setupGoProject(t)
	// Create a skills/ marker so findContentRepoRoot works
	os.MkdirAll(filepath.Join(root, "skills"), 0755)
	withFakeRepoRoot(t, root)

	createCmd.SetArgs([]string{"skills", "my-test-skill"})
	err := createCmd.RunE(createCmd, []string{"skills", "my-test-skill"})
	if err != nil {
		t.Fatalf("create skill should succeed: %v", err)
	}

	// Verify the directory was created
	dest := filepath.Join(root, "local", "skills", "my-test-skill")
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Fatalf("expected directory %s to exist", dest)
	}

	// Verify .nesco.yaml was written
	meta, err := metadata.Load(dest)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected .nesco.yaml to exist")
	}
	if meta.Name != "my-test-skill" {
		t.Errorf("expected name %q, got %q", "my-test-skill", meta.Name)
	}
	if meta.ID == "" {
		t.Error("expected non-empty ID")
	}
	if meta.CreatedAt == nil {
		t.Error("expected CreatedAt to be set")
	}
}

func TestCreateWritesNescoYaml(t *testing.T) {
	root := setupGoProject(t)
	os.MkdirAll(filepath.Join(root, "skills"), 0755)
	withFakeRepoRoot(t, root)

	createCmd.SetArgs([]string{"skills", "yaml-test-skill"})
	err := createCmd.RunE(createCmd, []string{"skills", "yaml-test-skill"})
	if err != nil {
		t.Fatalf("create should succeed: %v", err)
	}

	dest := filepath.Join(root, "local", "skills", "yaml-test-skill")
	meta, err := metadata.Load(dest)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected .nesco.yaml to exist")
	}
	if meta.Name != "yaml-test-skill" {
		t.Errorf("expected Name %q, got %q", "yaml-test-skill", meta.Name)
	}
	if meta.CreatedAt == nil {
		t.Error("expected CreatedAt to be set")
	}
	if meta.ImportedAt != nil {
		t.Errorf("expected ImportedAt to be nil for created items, got %v", meta.ImportedAt)
	}
}

func TestCreateRuleRequiresProvider(t *testing.T) {
	root := setupGoProject(t)
	os.MkdirAll(filepath.Join(root, "skills"), 0755)
	withFakeRepoRoot(t, root)

	err := createCmd.RunE(createCmd, []string{"rules", "my-rule"})
	if err == nil {
		t.Fatal("create rules without --provider should fail")
	}
}

func TestCreateFailsIfExists(t *testing.T) {
	root := setupGoProject(t)
	os.MkdirAll(filepath.Join(root, "skills"), 0755)
	withFakeRepoRoot(t, root)

	// Pre-create the directory
	dest := filepath.Join(root, "local", "skills", "existing-skill")
	os.MkdirAll(dest, 0755)

	err := createCmd.RunE(createCmd, []string{"skills", "existing-skill"})
	if err == nil {
		t.Fatal("create should fail if item already exists")
	}
}

func TestCreateWithProvider(t *testing.T) {
	root := setupGoProject(t)
	os.MkdirAll(filepath.Join(root, "skills"), 0755)
	withFakeRepoRoot(t, root)

	createCmd.Flags().Set("provider", "claude-code")
	t.Cleanup(func() { createCmd.Flags().Set("provider", "") })

	err := createCmd.RunE(createCmd, []string{"rules", "my-rule"})
	if err != nil {
		t.Fatalf("create rule with --provider should succeed: %v", err)
	}

	// Verify provider-specific path: local/rules/claude-code/my-rule/
	dest := filepath.Join(root, "local", "rules", "claude-code", "my-rule")
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Fatalf("expected directory %s to exist", dest)
	}

	// Verify metadata
	meta, err := metadata.Load(dest)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected .nesco.yaml to exist")
	}
	if meta.Name != "my-rule" {
		t.Errorf("expected name %q, got %q", "my-rule", meta.Name)
	}
}
