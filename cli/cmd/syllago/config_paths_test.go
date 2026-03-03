package main

import (
	"os"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// withTempHome sets HOME to a temp dir for the duration of the test.
func withTempHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	return tmp
}

func TestConfigPathsSetAndShow(t *testing.T) {
	withTempHome(t)
	stdout, _ := output.SetForTest(t)

	// Set a per-type path
	configPathsSetCmd.Flags().Set("type", "skills")
	configPathsSetCmd.Flags().Set("path", "/custom/skills")
	configPathsSetCmd.Flags().Set("base-dir", "")
	if err := configPathsSetCmd.RunE(configPathsSetCmd, []string{"claude-code"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !strings.Contains(stdout.String(), "Set claude-code skills path") {
		t.Errorf("expected confirmation, got: %s", stdout.String())
	}

	// Verify round-trip via show
	stdout.Reset()
	configPathsShowCmd.Flags().Set("provider", "")
	if err := configPathsShowCmd.RunE(configPathsShowCmd, nil); err != nil {
		t.Fatalf("show: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "claude-code") || !strings.Contains(out, "/custom/skills") {
		t.Errorf("show should display override, got: %s", out)
	}
}

func TestConfigPathsSetBaseDir(t *testing.T) {
	withTempHome(t)
	stdout, _ := output.SetForTest(t)

	configPathsSetCmd.Flags().Set("base-dir", "/custom/base")
	configPathsSetCmd.Flags().Set("type", "")
	configPathsSetCmd.Flags().Set("path", "")
	if err := configPathsSetCmd.RunE(configPathsSetCmd, []string{"claude-code"}); err != nil {
		t.Fatalf("set base-dir: %v", err)
	}
	if !strings.Contains(stdout.String(), "base-dir") {
		t.Errorf("expected base-dir confirmation, got: %s", stdout.String())
	}

	cfg, _ := config.LoadGlobal()
	if cfg.ProviderPaths["claude-code"].BaseDir != "/custom/base" {
		t.Errorf("BaseDir = %q, want /custom/base", cfg.ProviderPaths["claude-code"].BaseDir)
	}
}

func TestConfigPathsSetRejectsRelativePath(t *testing.T) {
	withTempHome(t)
	output.SetForTest(t)

	configPathsSetCmd.Flags().Set("base-dir", "relative/path")
	configPathsSetCmd.Flags().Set("type", "")
	configPathsSetCmd.Flags().Set("path", "")
	err := configPathsSetCmd.RunE(configPathsSetCmd, []string{"claude-code"})
	if err == nil {
		t.Fatal("expected error for relative path")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("error should mention absolute path, got: %v", err)
	}
}

func TestConfigPathsSetRejectsRelativeTypePath(t *testing.T) {
	withTempHome(t)
	output.SetForTest(t)

	configPathsSetCmd.Flags().Set("base-dir", "")
	configPathsSetCmd.Flags().Set("type", "skills")
	configPathsSetCmd.Flags().Set("path", "relative/skills")
	err := configPathsSetCmd.RunE(configPathsSetCmd, []string{"claude-code"})
	if err == nil {
		t.Fatal("expected error for relative type path")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("error should mention absolute path, got: %v", err)
	}
}

func TestConfigPathsSetRejectsInvalidContentType(t *testing.T) {
	withTempHome(t)
	output.SetForTest(t)

	configPathsSetCmd.Flags().Set("base-dir", "")
	configPathsSetCmd.Flags().Set("type", "invalid-type")
	configPathsSetCmd.Flags().Set("path", "/valid/path")
	err := configPathsSetCmd.RunE(configPathsSetCmd, []string{"claude-code"})
	if err == nil {
		t.Fatal("expected error for invalid content type")
	}
	if !strings.Contains(err.Error(), "unknown content type") {
		t.Errorf("error should mention unknown content type, got: %v", err)
	}
}

func TestConfigPathsSetAcceptsTilde(t *testing.T) {
	withTempHome(t)
	output.SetForTest(t)

	configPathsSetCmd.Flags().Set("base-dir", "~/custom/base")
	configPathsSetCmd.Flags().Set("type", "")
	configPathsSetCmd.Flags().Set("path", "")
	err := configPathsSetCmd.RunE(configPathsSetCmd, []string{"claude-code"})
	if err != nil {
		t.Fatalf("tilde paths should be accepted: %v", err)
	}
}

func TestConfigPathsClear(t *testing.T) {
	withTempHome(t)
	output.SetForTest(t)

	// First set something
	configPathsSetCmd.Flags().Set("base-dir", "/custom/base")
	configPathsSetCmd.Flags().Set("type", "skills")
	configPathsSetCmd.Flags().Set("path", "/custom/skills")
	configPathsSetCmd.RunE(configPathsSetCmd, []string{"claude-code"})

	// Clear just the type
	configPathsClearCmd.Flags().Set("type", "skills")
	if err := configPathsClearCmd.RunE(configPathsClearCmd, []string{"claude-code"}); err != nil {
		t.Fatalf("clear type: %v", err)
	}

	cfg, _ := config.LoadGlobal()
	ppc := cfg.ProviderPaths["claude-code"]
	if _, ok := ppc.Paths["skills"]; ok {
		t.Error("skills path should be cleared")
	}
	if ppc.BaseDir != "/custom/base" {
		t.Error("base-dir should be preserved when clearing a type")
	}
}

func TestConfigPathsClearAll(t *testing.T) {
	withTempHome(t)
	output.SetForTest(t)

	// Set something
	configPathsSetCmd.Flags().Set("base-dir", "/custom/base")
	configPathsSetCmd.Flags().Set("type", "")
	configPathsSetCmd.Flags().Set("path", "")
	configPathsSetCmd.RunE(configPathsSetCmd, []string{"claude-code"})

	// Clear entire provider
	configPathsClearCmd.Flags().Set("type", "")
	if err := configPathsClearCmd.RunE(configPathsClearCmd, []string{"claude-code"}); err != nil {
		t.Fatalf("clear all: %v", err)
	}

	cfg, _ := config.LoadGlobal()
	if len(cfg.ProviderPaths) > 0 {
		t.Errorf("ProviderPaths should be empty after clearing, got %v", cfg.ProviderPaths)
	}
}

func TestConfigPathsClearNonexistent(t *testing.T) {
	withTempHome(t)
	output.SetForTest(t)

	configPathsClearCmd.Flags().Set("type", "")
	err := configPathsClearCmd.RunE(configPathsClearCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("clearing nonexistent provider should fail")
	}
}

func TestConfigPathsShowEmpty(t *testing.T) {
	withTempHome(t)
	stdout, _ := output.SetForTest(t)

	configPathsShowCmd.Flags().Set("provider", "")
	if err := configPathsShowCmd.RunE(configPathsShowCmd, nil); err != nil {
		t.Fatalf("show empty: %v", err)
	}
	if !strings.Contains(stdout.String(), "No path overrides") {
		t.Errorf("expected empty message, got: %s", stdout.String())
	}
}

func TestConfigPathsShowFilterByProvider(t *testing.T) {
	withTempHome(t)
	stdout, _ := output.SetForTest(t)

	// Set overrides for two providers
	configPathsSetCmd.Flags().Set("base-dir", "/claude/base")
	configPathsSetCmd.Flags().Set("type", "")
	configPathsSetCmd.Flags().Set("path", "")
	configPathsSetCmd.RunE(configPathsSetCmd, []string{"claude-code"})

	configPathsSetCmd.Flags().Set("base-dir", "/cursor/base")
	configPathsSetCmd.RunE(configPathsSetCmd, []string{"cursor"})

	// Show only claude-code
	stdout.Reset()
	configPathsShowCmd.Flags().Set("provider", "claude-code")
	if err := configPathsShowCmd.RunE(configPathsShowCmd, nil); err != nil {
		t.Fatalf("show filtered: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "claude-code") {
		t.Error("should show claude-code")
	}
	if strings.Contains(out, "cursor") {
		t.Error("should not show cursor when filtering by claude-code")
	}
}

func TestConfigPathsSetRequiresTypeWithPath(t *testing.T) {
	withTempHome(t)
	output.SetForTest(t)

	// Path without type
	configPathsSetCmd.Flags().Set("base-dir", "")
	configPathsSetCmd.Flags().Set("type", "")
	configPathsSetCmd.Flags().Set("path", "/some/path")
	err := configPathsSetCmd.RunE(configPathsSetCmd, []string{"claude-code"})
	if err == nil {
		t.Fatal("--path without --type should fail")
	}
}
