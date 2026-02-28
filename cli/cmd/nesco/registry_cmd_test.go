package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/config"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/OpenScribbler/nesco/cli/internal/registry"
)

func TestRegistryAddRejectsDisallowedURL(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	cfg := &config.Config{
		Providers:         []string{"claude-code"},
		AllowedRegistries: []string{"https://github.com/allowed/only.git"},
	}
	if err := config.Save(tmp, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	err := registryAddCmd.RunE(registryAddCmd, []string{"https://github.com/not/allowed.git"})
	if err == nil {
		t.Fatal("expected error for disallowed URL, got nil")
	}
	if !strings.Contains(err.Error(), "allowedRegistries") {
		t.Errorf("error %q does not mention allowedRegistries", err.Error())
	}
}

func TestRegistryAddAllowsURLWhenNoPolicy(t *testing.T) {
	// When AllowedRegistries is empty (nil), any URL should pass the check.
	// We test only up to the clone step — clone will fail (no network/git),
	// but the important thing is the error is NOT about allowedRegistries.
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	cfg := &config.Config{
		Providers: []string{"claude-code"},
		// AllowedRegistries is empty — all URLs permitted
	}
	if err := config.Save(tmp, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	err := registryAddCmd.RunE(registryAddCmd, []string{"https://github.com/any/repo.git"})
	// The call will fail at git clone (no network), but must NOT fail with allowedRegistries error
	if err != nil && strings.Contains(err.Error(), "allowedRegistries") {
		t.Errorf("should not reject URL when allowedRegistries is empty, got: %v", err)
	}
}

func TestRegistryAddAllowsURLInPolicy(t *testing.T) {
	// When a URL is explicitly in AllowedRegistries, it should pass the check.
	// Again, clone will fail but must NOT fail with allowedRegistries error.
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	cfg := &config.Config{
		Providers:         []string{"claude-code"},
		AllowedRegistries: []string{"https://github.com/allowed/repo.git"},
	}
	if err := config.Save(tmp, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	err := registryAddCmd.RunE(registryAddCmd, []string{"https://github.com/allowed/repo.git"})
	// Clone will fail but NOT with allowedRegistries error
	if err != nil && strings.Contains(err.Error(), "allowedRegistries") {
		t.Errorf("should not reject allowed URL, got: %v", err)
	}
}

// TestRegistryListShowsManifest verifies that registry list output includes
// manifest version and description when a registry.yaml is present in the clone.
func TestRegistryListShowsManifest(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	// Create a fake registry clone dir with a registry.yaml
	cacheDir, err := registry.CacheDir()
	if err != nil {
		t.Fatalf("registry.CacheDir: %v", err)
	}
	registryClone := filepath.Join(cacheDir, "test-reg-43")
	os.MkdirAll(registryClone, 0755)
	defer os.RemoveAll(registryClone)

	manifestContent := "name: test-reg-43\ndescription: A test registry\nversion: \"1.2.3\"\n"
	if err := os.WriteFile(filepath.Join(registryClone, "registry.yaml"), []byte(manifestContent), 0644); err != nil {
		t.Fatalf("WriteFile registry.yaml: %v", err)
	}

	// Create a config with the test registry
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "test-reg-43", URL: "https://github.com/example/test-reg-43.git"},
		},
	}
	if err := config.Save(tmp, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Capture output
	stdout, _ := output.SetForTest(t)

	if err := registryListCmd.RunE(registryListCmd, nil); err != nil {
		t.Fatalf("registryListCmd.RunE: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "1.2.3") {
		t.Errorf("expected version '1.2.3' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "A test registry") {
		t.Errorf("expected description 'A test registry' in output, got:\n%s", got)
	}
}

// TestRegistryAddExpandsAlias verifies that alias expansion happens before
// the allowedRegistries check so the expanded full URL is what gets checked.
func TestRegistryAddExpandsAlias(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	// Config restricts to the expanded nesco-tools URL
	cfg := &config.Config{
		Providers:         []string{"claude-code"},
		AllowedRegistries: []string{"https://github.com/OpenScribbler/nesco-tools.git"},
	}
	if err := config.Save(tmp, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Pass the short alias — it should expand and pass the allowedRegistries check,
	// then fail at git clone (no network), but NOT with an allowedRegistries error.
	err := registryAddCmd.RunE(registryAddCmd, []string{"nesco-tools"})
	if err != nil && strings.Contains(err.Error(), "allowedRegistries") {
		t.Errorf("alias should expand before allowedRegistries check, got: %v", err)
	}
}
