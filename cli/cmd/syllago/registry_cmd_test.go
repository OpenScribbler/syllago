package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
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
	// Temporarily inject a test alias so we can exercise the expansion path.
	const testAlias = "test-alias"
	const testURL = "https://github.com/acme/test-tools.git"
	registry.KnownAliases[testAlias] = testURL
	defer delete(registry.KnownAliases, testAlias)

	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	// Config restricts to the expanded alias URL
	cfg := &config.Config{
		Providers:         []string{"claude-code"},
		AllowedRegistries: []string{testURL},
	}
	if err := config.Save(tmp, cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Pass the short alias — it should expand and pass the allowedRegistries check,
	// then fail at git clone (no network), but NOT with an allowedRegistries error.
	err := registryAddCmd.RunE(registryAddCmd, []string{testAlias})
	if err != nil && strings.Contains(err.Error(), "allowedRegistries") {
		t.Errorf("alias should expand before allowedRegistries check, got: %v", err)
	}
}

// chdirTo changes to dir and restores the original cwd on cleanup.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// resetRegistryCreateFlags resets the flags mutated by registry create tests.
func resetRegistryCreateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		registryCreateCmd.Flags().Set("new", "")
		registryCreateCmd.Flags().Set("description", "")
		registryCreateCmd.Flags().Set("no-git", "false")
		registryCreateCmd.Flags().Set("from-native", "false")
	})
}

func TestRunRegistryCreateNew_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	resetRegistryCreateFlags(t)

	stdout, _ := output.SetForTest(t)
	registryCreateCmd.Flags().Set("new", "my-registry")
	registryCreateCmd.Flags().Set("no-git", "true") // avoid git dependency in unit tests

	if err := registryCreateCmd.RunE(registryCreateCmd, nil); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	dir := filepath.Join(tmp, "my-registry")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("expected scaffold dir %s, stat err=%v", dir, err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Created registry scaffold at") {
		t.Errorf("expected scaffold message, got: %s", out)
	}
	if !strings.Contains(out, "Next steps:") {
		t.Errorf("expected next-steps message, got: %s", out)
	}
}

func TestRunRegistryCreateNew_WithDescription(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	resetRegistryCreateFlags(t)

	_, _ = output.SetForTest(t)
	registryCreateCmd.Flags().Set("new", "desc-registry")
	registryCreateCmd.Flags().Set("description", "Team rules")
	registryCreateCmd.Flags().Set("no-git", "true")

	if err := registryCreateCmd.RunE(registryCreateCmd, nil); err != nil {
		t.Fatalf("expected success with description, got: %v", err)
	}

	manifestPath := filepath.Join(tmp, "desc-registry", "registry.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	if !strings.Contains(string(data), "Team rules") {
		t.Errorf("expected description in manifest, got: %s", data)
	}
}

func TestRunRegistryCreateNew_InvalidName(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	resetRegistryCreateFlags(t)

	_, _ = output.SetForTest(t)
	// Path separators and spaces are invalid.
	registryCreateCmd.Flags().Set("new", "bad name/with-slash")
	registryCreateCmd.Flags().Set("no-git", "true")

	err := registryCreateCmd.RunE(registryCreateCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid registry name")
	}
	if !strings.Contains(err.Error(), "invalid characters") {
		t.Errorf("expected 'invalid characters' in error, got: %v", err)
	}
}

func TestRunRegistryCreateNew_DirAlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	resetRegistryCreateFlags(t)

	// Pre-create the target dir.
	if err := os.MkdirAll(filepath.Join(tmp, "existing"), 0755); err != nil {
		t.Fatal(err)
	}

	_, _ = output.SetForTest(t)
	registryCreateCmd.Flags().Set("new", "existing")
	registryCreateCmd.Flags().Set("no-git", "true")

	err := registryCreateCmd.RunE(registryCreateCmd, nil)
	if err == nil {
		t.Fatal("expected error when target dir exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

func TestRunRegistryCreateNew_NoGitFlagSkipsGitInit(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	resetRegistryCreateFlags(t)

	stdout, _ := output.SetForTest(t)
	registryCreateCmd.Flags().Set("new", "no-git-registry")
	registryCreateCmd.Flags().Set("no-git", "true")

	if err := registryCreateCmd.RunE(registryCreateCmd, nil); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// With no-git: no "Initialized git repository" message, and no .git dir.
	out := stdout.String()
	if strings.Contains(out, "Initialized git repository") {
		t.Errorf("expected no git init with --no-git, got: %s", out)
	}
	if _, err := os.Stat(filepath.Join(tmp, "no-git-registry", ".git")); !os.IsNotExist(err) {
		t.Errorf("expected no .git dir with --no-git, stat err=%v", err)
	}
	// Instructions should mention manual git init.
	if !strings.Contains(out, "git init && git add") {
		t.Errorf("expected manual-git instructions in output, got: %s", out)
	}
}

func TestRunRegistryCreateNew_GitInitSuccess(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	tmp := t.TempDir()
	chdirTo(t, tmp)
	resetRegistryCreateFlags(t)

	stdout, _ := output.SetForTest(t)
	registryCreateCmd.Flags().Set("new", "git-registry")
	// no-git defaults to false, so git init should happen.

	if err := registryCreateCmd.RunE(registryCreateCmd, nil); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Initialized git repository") {
		t.Errorf("expected git init success message, got: %s", out)
	}
	if _, err := os.Stat(filepath.Join(tmp, "git-registry", ".git")); err != nil {
		t.Errorf("expected .git dir, stat err=%v", err)
	}
}

func TestRunRegistryCreateNew_AlreadyInGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	tmp := t.TempDir()

	// Pre-init tmp as a git repo so IsInsideGitRepo returns true.
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "Test"},
	} {
		c := exec.Command("git", args...)
		c.Dir = tmp
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	chdirTo(t, tmp)
	resetRegistryCreateFlags(t)

	stdout, _ := output.SetForTest(t)
	registryCreateCmd.Flags().Set("new", "nested-registry")
	// no-git left false, but since we're already in a git repo, init is skipped.

	if err := registryCreateCmd.RunE(registryCreateCmd, nil); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "already inside a git repo") {
		t.Errorf("expected 'already inside a git repo' note, got: %s", out)
	}
}

func TestRunRegistryCreateNew_NoFlagsShowsHelp(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)
	resetRegistryCreateFlags(t)

	_, _ = output.SetForTest(t)
	// Neither --new nor --from-native set.
	if err := registryCreateCmd.RunE(registryCreateCmd, nil); err != nil {
		t.Fatalf("expected help output with no flags, got error: %v", err)
	}
}
