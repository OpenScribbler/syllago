package detectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestMonorepoDetectsPnpmWorkspace(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	pnpmYaml := `packages:
  - 'packages/*'
`
	os.WriteFile(filepath.Join(tmp, "pnpm-workspace.yaml"), []byte(pnpmYaml), 0644)

	// Create workspace packages with package.json files
	for _, pkg := range []string{"ui", "api"} {
		dir := filepath.Join(tmp, "packages", pkg)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "`+pkg+`"}`), 0644)
	}

	det := MonorepoStructure{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for pnpm workspace")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatConventions {
		t.Errorf("category = %q, want %q", ts.Category, model.CatConventions)
	}
	if !strings.Contains(ts.Body, "pnpm") {
		t.Errorf("body should mention pnpm, got: %s", ts.Body)
	}
}

func TestMonorepoDetectsNpmWorkspaces(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	pkg := `{
  "name": "my-monorepo",
  "workspaces": ["packages/*"]
}`
	os.WriteFile(filepath.Join(tmp, "package.json"), []byte(pkg), 0644)

	// Create a workspace package
	pkgDir := filepath.Join(tmp, "packages", "shared")
	os.MkdirAll(pkgDir, 0755)
	os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"name": "shared"}`), 0644)

	det := MonorepoStructure{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for npm workspaces")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatConventions {
		t.Errorf("category = %q, want %q", ts.Category, model.CatConventions)
	}
}

func TestMonorepoNotDetected(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Single-package project — no workspaces field
	pkg := `{
  "name": "single-app",
  "version": "1.0.0"
}`
	os.WriteFile(filepath.Join(tmp, "package.json"), []byte(pkg), 0644)

	det := MonorepoStructure{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for single-package project, got %d", len(sections))
	}
}

func TestMonorepoSkipsEmptyDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	det := MonorepoStructure{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if sections != nil {
		t.Errorf("expected nil sections for empty dir, got %d", len(sections))
	}
}
