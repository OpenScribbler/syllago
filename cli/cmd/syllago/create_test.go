package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// withGlobalContentDir sets catalog.GlobalContentDirOverride for test isolation
// and restores the original value via t.Cleanup.
func withGlobalContentDir(t *testing.T, dir string) {
	t.Helper()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })
}

func TestCreateSkill(t *testing.T) {
	globalDir := t.TempDir()
	withGlobalContentDir(t, globalDir)

	createCmd.SetArgs([]string{"skills", "my-test-skill"})
	err := createCmd.RunE(createCmd, []string{"skills", "my-test-skill"})
	if err != nil {
		t.Fatalf("create skill should succeed: %v", err)
	}

	// Verify the directory was created under globalDir
	dest := filepath.Join(globalDir, "skills", "my-test-skill")
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Fatalf("expected directory %s to exist", dest)
	}

	// Verify .syllago.yaml was written
	meta, err := metadata.Load(dest)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected .syllago.yaml to exist")
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

func TestCreateWritesSyllagoYaml(t *testing.T) {
	globalDir := t.TempDir()
	withGlobalContentDir(t, globalDir)

	createCmd.SetArgs([]string{"skills", "yaml-test-skill"})
	err := createCmd.RunE(createCmd, []string{"skills", "yaml-test-skill"})
	if err != nil {
		t.Fatalf("create should succeed: %v", err)
	}

	dest := filepath.Join(globalDir, "skills", "yaml-test-skill")
	meta, err := metadata.Load(dest)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected .syllago.yaml to exist")
	}
	if meta.Name != "yaml-test-skill" {
		t.Errorf("expected Name %q, got %q", "yaml-test-skill", meta.Name)
	}
	if meta.CreatedAt == nil {
		t.Error("expected CreatedAt to be set")
	}
	if meta.AddedAt != nil {
		t.Errorf("expected AddedAt to be nil for created items, got %v", meta.AddedAt)
	}
}

func TestCreateRuleRequiresProvider(t *testing.T) {
	globalDir := t.TempDir()
	withGlobalContentDir(t, globalDir)

	err := createCmd.RunE(createCmd, []string{"rules", "my-rule"})
	if err == nil {
		t.Fatal("create rules without --provider should fail")
	}
}

func TestCreateFailsIfExists(t *testing.T) {
	globalDir := t.TempDir()
	withGlobalContentDir(t, globalDir)

	// Pre-create the directory
	dest := filepath.Join(globalDir, "skills", "existing-skill")
	os.MkdirAll(dest, 0755)

	err := createCmd.RunE(createCmd, []string{"skills", "existing-skill"})
	if err == nil {
		t.Fatal("create should fail if item already exists")
	}
}

func TestCreateRejectsInvalidNames(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"path traversal", "../../evil"},
		{"leading dash", "-bad"},
		{"dots", "foo.bar"},
		{"spaces", "has spaces"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateCreateArgs("skills", tc.input, "")
			if err == nil {
				t.Fatalf("expected error for name %q", tc.input)
			}
		})
	}
}

func TestCreateWithProvider(t *testing.T) {
	globalDir := t.TempDir()
	withGlobalContentDir(t, globalDir)

	createCmd.Flags().Set("provider", "claude-code")
	t.Cleanup(func() { createCmd.Flags().Set("provider", "") })

	err := createCmd.RunE(createCmd, []string{"rules", "my-rule"})
	if err != nil {
		t.Fatalf("create rule with --provider should succeed: %v", err)
	}

	// Verify provider-specific path: <globalDir>/rules/claude-code/my-rule/
	dest := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Fatalf("expected directory %s to exist", dest)
	}

	// Verify metadata
	meta, err := metadata.Load(dest)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected .syllago.yaml to exist")
	}
	if meta.Name != "my-rule" {
		t.Errorf("expected name %q, got %q", "my-rule", meta.Name)
	}
}
