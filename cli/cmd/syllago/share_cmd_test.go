package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/promote"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestShareCmdRegisters(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "share <name>" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected share command registered on rootCmd")
	}
}

func TestShareCmdHasToFlag(t *testing.T) {
	flag := shareCmd.Flags().Lookup("to")
	if flag == nil {
		t.Fatal("expected --to flag on share command")
	}
	if flag.DefValue != "" {
		t.Errorf("expected empty default for --to, got %q", flag.DefValue)
	}
}

func TestShareCmdValidatesArgs(t *testing.T) {
	shareCmd.SilenceUsage = true
	shareCmd.SilenceErrors = true

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, true},
		{"two args", []string{"first", "second"}, true},
		// One arg is correct — arg validation passes; RunE will fail later.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := shareCmd.Args(shareCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestFindLibraryItem_NotFound(t *testing.T) {
	_, _ = output.SetForTest(t)

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "other-skill", Type: catalog.Skills, Source: "global"},
		},
	}

	_, err := findLibraryItem(cat, "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for item not found")
	}
	if !strings.Contains(err.Error(), "no item named") {
		t.Errorf("expected 'no item named' in error, got: %v", err)
	}
}

func TestFindLibraryItem_Found(t *testing.T) {
	_, _ = output.SetForTest(t)

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-skill", Type: catalog.Skills, Source: "global"},
		},
	}

	item, err := findLibraryItem(cat, "my-skill", "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if item.Name != "my-skill" {
		t.Errorf("expected item named 'my-skill', got %q", item.Name)
	}
}

func TestFindLibraryItem_IgnoresNonGlobal(t *testing.T) {
	_, _ = output.SetForTest(t)

	// Item exists but is not in the global library (e.g. shared or registry).
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-skill", Type: catalog.Skills, Source: "shared"},
			{Name: "my-skill", Type: catalog.Skills, Source: "registry-foo"},
		},
	}

	_, err := findLibraryItem(cat, "my-skill", "")
	if err == nil {
		t.Fatal("expected error: non-global items should not be found")
	}
	if !strings.Contains(err.Error(), "no item named") {
		t.Errorf("expected 'no item named' in error, got: %v", err)
	}
}

func TestFindLibraryItem_AmbiguousWithoutTypeFilter(t *testing.T) {
	_, _ = output.SetForTest(t)

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-item", Type: catalog.Skills, Source: "global"},
			{Name: "my-item", Type: catalog.Rules, Source: "global"},
		},
	}

	_, err := findLibraryItem(cat, "my-item", "")
	if err == nil {
		t.Fatal("expected error for ambiguous name")
	}
	if !strings.Contains(err.Error(), "multiple types") {
		t.Errorf("expected 'multiple types' in error, got: %v", err)
	}
}

func TestFindLibraryItem_TypeFilterDisambiguates(t *testing.T) {
	_, _ = output.SetForTest(t)

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-item", Type: catalog.Skills, Source: "global"},
			{Name: "my-item", Type: catalog.Rules, Source: "global"},
		},
	}

	item, err := findLibraryItem(cat, "my-item", string(catalog.Skills))
	if err != nil {
		t.Fatalf("expected no error with type filter, got: %v", err)
	}
	if item.Type != catalog.Skills {
		t.Errorf("expected skills type, got %q", item.Type)
	}
}

func TestShareItemNotFound(t *testing.T) {
	lib := setupConvertLibrary(t) // reuse: creates a skill named "my-skill"
	withConvertLibrary(t, lib)

	_, _ = output.SetForTest(t)

	shareCmd.Flags().Set("type", "")
	defer shareCmd.Flags().Set("type", "")

	err := shareCmd.RunE(shareCmd, []string{"nonexistent-item"})
	if err == nil {
		t.Fatal("expected error for nonexistent item")
	}
	if !strings.Contains(err.Error(), "no item named") {
		t.Errorf("expected 'no item named' in error, got: %v", err)
	}
}

// withNoRepoRoot clears both the ldflags-injected repoRoot var and the
// findProjectRoot override so findContentRepoRoot errors with "could not find
// syllago content repository".
func withNoRepoRoot(t *testing.T) {
	t.Helper()
	origRepo := repoRoot
	repoRoot = ""
	t.Cleanup(func() { repoRoot = origRepo })

	origFind := findProjectRoot
	findProjectRoot = func() (string, error) {
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { findProjectRoot = origFind })
}

// initGitRepo runs `git init` + an initial commit so promote.Promote's
// isTreeDirty check succeeds with a clean tree.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init", "-q"},
	}
	for _, args := range cmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestShare_NoContentRepo(t *testing.T) {
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)
	withNoRepoRoot(t)

	_, _ = output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	defer shareCmd.Flags().Set("type", "")

	err := shareCmd.RunE(shareCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error when no content repo is found")
	}
	if !strings.Contains(err.Error(), "could not find syllago") {
		t.Errorf("expected 'could not find syllago' in error, got: %v", err)
	}
}

func TestShare_ToUnclonedRegistryReturnsError(t *testing.T) {
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)

	// Use a valid but empty content repo so findContentRepoRoot succeeds.
	repo := t.TempDir()
	withFakeRepoRoot(t, repo)
	origRepo := repoRoot
	repoRoot = ""
	t.Cleanup(func() { repoRoot = origRepo })

	// Isolate the registry cache so "nope-registry" is guaranteed missing.
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = t.TempDir()
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	_, _ = output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "nope-registry")
	t.Cleanup(func() {
		shareCmd.Flags().Set("type", "")
		shareCmd.Flags().Set("to", "")
	})

	err := shareCmd.RunE(shareCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error when target registry is not cloned")
	}
	if !strings.Contains(err.Error(), "share to registry failed") {
		t.Errorf("expected 'share to registry failed' in error, got: %v", err)
	}
}

func TestShare_PromoteErrorsOnMissingMetadata(t *testing.T) {
	// Skill fixture has SKILL.md but no .syllago.yaml, so promote.Promote
	// returns "item has no .syllago.yaml metadata" and runShare wraps it.
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)

	repo := t.TempDir()
	initGitRepo(t, repo)
	withFakeRepoRoot(t, repo)
	origRepo := repoRoot
	repoRoot = ""
	t.Cleanup(func() { repoRoot = origRepo })

	_, _ = output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "")
	t.Cleanup(func() {
		shareCmd.Flags().Set("type", "")
		shareCmd.Flags().Set("to", "")
	})

	err := shareCmd.RunE(shareCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error when item has no .syllago.yaml metadata")
	}
	if !strings.Contains(err.Error(), "sharing failed") {
		t.Errorf("expected 'sharing failed' in error, got: %v", err)
	}
}

// TestShare_DisplaysStagingMessage verifies that the pre-share status line
// makes it to stdout before promote.Promote returns its error.
func TestShare_DisplaysStagingMessage(t *testing.T) {
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)

	repo := t.TempDir()
	initGitRepo(t, repo)
	withFakeRepoRoot(t, repo)
	origRepo := repoRoot
	repoRoot = ""
	t.Cleanup(func() { repoRoot = origRepo })

	stdout, _ := output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "")
	t.Cleanup(func() {
		shareCmd.Flags().Set("type", "")
		shareCmd.Flags().Set("to", "")
	})

	_ = shareCmd.RunE(shareCmd, []string{"my-skill"})

	if !strings.Contains(stdout.String(), "Staging changes") {
		t.Errorf("expected 'Staging changes' message in stdout, got: %s", stdout.String())
	}
}

// TestShare_ToRegistryDisplaysPublishMessage verifies the pre-registry-push
// status line is emitted before the error.
func TestShare_ToRegistryDisplaysPublishMessage(t *testing.T) {
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)

	repo := t.TempDir()
	withFakeRepoRoot(t, repo)
	origRepo := repoRoot
	repoRoot = ""
	t.Cleanup(func() { repoRoot = origRepo })

	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = t.TempDir()
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	stdout, _ := output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "nope-registry")
	t.Cleanup(func() {
		shareCmd.Flags().Set("type", "")
		shareCmd.Flags().Set("to", "")
	})

	_ = shareCmd.RunE(shareCmd, []string{"my-skill"})

	if !strings.Contains(stdout.String(), "Sharing to registry") {
		t.Errorf("expected 'Sharing to registry' message in stdout, got: %s", stdout.String())
	}
}

// Prevent unused-import warning when only some paths hit filepath.
var _ = filepath.Join

// withStubbedPromote replaces promoteFunc with a stub returning the given result.
func withStubbedPromote(t *testing.T, result *promote.Result, err error) {
	t.Helper()
	orig := promoteFunc
	promoteFunc = func(string, catalog.ContentItem, bool) (*promote.Result, error) {
		return result, err
	}
	t.Cleanup(func() { promoteFunc = orig })
}

// withStubbedPromoteToRegistry replaces promoteToRegistryFunc with a stub.
func withStubbedPromoteToRegistry(t *testing.T, result *promote.RegistryResult, err error) {
	t.Helper()
	orig := promoteToRegistryFunc
	promoteToRegistryFunc = func(string, string, catalog.ContentItem, bool) (*promote.RegistryResult, error) {
		return result, err
	}
	t.Cleanup(func() { promoteToRegistryFunc = orig })
}

// setupShareHappyPath wires a library, a fake repo root, and a successful
// promote stub so the happy-path branches in runShare can be exercised.
func setupShareHappyPath(t *testing.T) {
	t.Helper()
	lib := setupConvertLibrary(t)
	withConvertLibrary(t, lib)

	repo := t.TempDir()
	withFakeRepoRoot(t, repo)
	t.Cleanup(func() {
		shareCmd.Flags().Set("type", "")
		shareCmd.Flags().Set("to", "")
	})
}

func TestShare_PromoteSuccessWithPRUrl(t *testing.T) {
	setupShareHappyPath(t)
	withStubbedPromote(t, &promote.Result{
		Branch: "syllago/promote/skills/my-skill",
		PRUrl:  "https://github.com/org/repo/pull/42",
	}, nil)

	stdout, _ := output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "")

	if err := shareCmd.RunE(shareCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Shared! PR: https://github.com/org/repo/pull/42") {
		t.Errorf("expected PR URL message, got: %s", out)
	}
}

func TestShare_PromoteSuccessWithCompareURL(t *testing.T) {
	setupShareHappyPath(t)
	withStubbedPromote(t, &promote.Result{
		Branch:     "syllago/promote/skills/my-skill",
		CompareURL: "https://github.com/org/repo/compare/main...branch",
	}, nil)

	stdout, _ := output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "")

	if err := shareCmd.RunE(shareCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Open a PR:") {
		t.Errorf("expected compare URL message, got: %s", out)
	}
}

func TestShare_PromoteSuccessPushOnly(t *testing.T) {
	setupShareHappyPath(t)
	withStubbedPromote(t, &promote.Result{
		Branch: "syllago/promote/skills/my-skill",
	}, nil)

	stdout, _ := output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "")

	if err := shareCmd.RunE(shareCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Shared! Branch") || !strings.Contains(out, "pushed.") {
		t.Errorf("expected push-only message, got: %s", out)
	}
}

func TestShare_PromoteSuccessJSON(t *testing.T) {
	setupShareHappyPath(t)
	withStubbedPromote(t, &promote.Result{
		Branch: "syllago/promote/skills/my-skill",
		PRUrl:  "https://github.com/org/repo/pull/42",
	}, nil)

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "")

	if err := shareCmd.RunE(shareCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	var got shareResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nstdout: %s", err, stdout.String())
	}
	if got.Name != "my-skill" || got.Branch == "" || got.PRUrl == "" {
		t.Errorf("unexpected JSON result: %+v", got)
	}
}

func TestShare_RegistrySuccessWithPRUrl(t *testing.T) {
	setupShareHappyPath(t)
	withStubbedPromoteToRegistry(t, &promote.RegistryResult{
		Branch: "syllago/promote/skills/my-skill",
		PRUrl:  "https://github.com/org/registry/pull/7",
	}, nil)

	stdout, _ := output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "my-registry")

	if err := shareCmd.RunE(shareCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Shared! PR: https://github.com/org/registry/pull/7") {
		t.Errorf("expected registry PR URL message, got: %s", out)
	}
}

func TestShare_RegistrySuccessWithCompareURL(t *testing.T) {
	setupShareHappyPath(t)
	withStubbedPromoteToRegistry(t, &promote.RegistryResult{
		Branch:     "syllago/promote/skills/my-skill",
		CompareURL: "https://github.com/org/registry/compare/main...branch",
	}, nil)

	stdout, _ := output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "my-registry")

	if err := shareCmd.RunE(shareCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Open a PR:") {
		t.Errorf("expected registry compare URL message, got: %s", out)
	}
}

func TestShare_RegistrySuccessPushOnly(t *testing.T) {
	setupShareHappyPath(t)
	withStubbedPromoteToRegistry(t, &promote.RegistryResult{
		Branch: "syllago/promote/skills/my-skill",
	}, nil)

	stdout, _ := output.SetForTest(t)
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "my-registry")

	if err := shareCmd.RunE(shareCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "pushed to registry") {
		t.Errorf("expected registry push-only message, got: %s", out)
	}
}

func TestShare_RegistrySuccessJSON(t *testing.T) {
	setupShareHappyPath(t)
	withStubbedPromoteToRegistry(t, &promote.RegistryResult{
		Branch: "syllago/promote/skills/my-skill",
		PRUrl:  "https://github.com/org/registry/pull/7",
	}, nil)

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	shareCmd.Flags().Set("type", "")
	shareCmd.Flags().Set("to", "my-registry")

	if err := shareCmd.RunE(shareCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	var got shareResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nstdout: %s", err, stdout.String())
	}
	if got.Registry != "my-registry" || got.Name != "my-skill" {
		t.Errorf("unexpected JSON result: %+v", got)
	}
}
