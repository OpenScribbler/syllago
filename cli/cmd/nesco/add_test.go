package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddRequiresType(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	os.MkdirAll(src, 0755)

	addCmd.Flags().Set("type", "")
	err := addCmd.RunE(addCmd, []string{src})
	if err == nil {
		t.Error("expected error when --type is missing")
	}
}

func TestAddRequiresProviderForNonUniversal(t *testing.T) {
	tmp := setupRepoWithSkills(t)
	src := filepath.Join(tmp, "source")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "test.md"), []byte("test"), 0644)

	addCmd.Flags().Set("type", "rules")
	addCmd.Flags().Set("provider", "")
	addCmd.Flags().Set("name", "test-rule")
	err := addCmd.RunE(addCmd, []string{src})
	if err == nil {
		t.Error("expected error when --provider is missing for rules")
	}
}

func TestAddCopiesContent(t *testing.T) {
	tmp := setupRepoWithSkills(t)
	src := filepath.Join(tmp, "source")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Test Skill"), 0644)

	addCmd.Flags().Set("type", "skills")
	addCmd.Flags().Set("provider", "")
	addCmd.Flags().Set("name", "my-test-skill")
	err := addCmd.RunE(addCmd, []string{src})
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	dest := filepath.Join(tmp, "my-tools", "skills", "my-test-skill")
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Error("SKILL.md should exist at destination")
	}
}

func TestAddGeneratesReadme(t *testing.T) {
	tmp := setupRepoWithSkills(t)
	src := filepath.Join(tmp, "source-no-readme")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# No README"), 0644)

	addCmd.Flags().Set("type", "skills")
	addCmd.Flags().Set("provider", "")
	addCmd.Flags().Set("name", "no-readme-skill")
	err := addCmd.RunE(addCmd, []string{src})
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	dest := filepath.Join(tmp, "my-tools", "skills", "no-readme-skill")
	if _, err := os.Stat(filepath.Join(dest, "README.md")); err != nil {
		t.Error("README.md should have been generated")
	}
}

func TestAddSkipsExistingReadme(t *testing.T) {
	tmp := setupRepoWithSkills(t)
	src := filepath.Join(tmp, "source-with-readme")
	os.MkdirAll(src, 0755)
	existing := "# My Custom README\n\nDo not overwrite.\n"
	os.WriteFile(filepath.Join(src, "README.md"), []byte(existing), 0644)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Has README"), 0644)

	addCmd.Flags().Set("type", "skills")
	addCmd.Flags().Set("provider", "")
	addCmd.Flags().Set("name", "has-readme-skill")
	err := addCmd.RunE(addCmd, []string{src})
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	dest := filepath.Join(tmp, "my-tools", "skills", "has-readme-skill")
	data, err := os.ReadFile(filepath.Join(dest, "README.md"))
	if err != nil {
		t.Fatal("README.md should exist at destination")
	}
	if string(data) != existing {
		t.Errorf("README.md was overwritten, got:\n%s", string(data))
	}
}

func TestAddRefusesExistingDest(t *testing.T) {
	tmp := setupRepoWithSkills(t)
	src := filepath.Join(tmp, "source")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Test"), 0644)

	// Pre-create destination
	dest := filepath.Join(tmp, "my-tools", "skills", "existing-skill")
	os.MkdirAll(dest, 0755)

	addCmd.Flags().Set("type", "skills")
	addCmd.Flags().Set("provider", "")
	addCmd.Flags().Set("name", "existing-skill")
	err := addCmd.RunE(addCmd, []string{src})
	if err == nil {
		t.Error("expected error when destination already exists")
	}
}

// setupRepoWithSkills creates a temp dir with a skills/ directory so findContentRepoRoot() works.
func setupRepoWithSkills(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "skills"), 0755)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	t.Cleanup(func() { os.Chdir(origDir) })

	return tmp
}
