package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
)

func TestResolveHookScripts_InlineCommand(t *testing.T) {
	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "echo lint"}]}`)
	item := catalog.ContentItem{Name: "test-hook", Path: t.TempDir()}

	result, _, err := resolveHookScripts(matcherGroup, item, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Inline command should be unchanged
	cmd := gjson.GetBytes(result, "hooks.0.command").String()
	if cmd != "echo lint" {
		t.Errorf("command changed: got %q", cmd)
	}
}

func TestResolveHookScripts_RelativeScript(t *testing.T) {
	// Create a hook item directory with a script
	itemDir := t.TempDir()
	os.WriteFile(filepath.Join(itemDir, "lint.sh"), []byte("#!/bin/bash\necho lint"), 0755)
	os.WriteFile(filepath.Join(itemDir, "hook.json"), []byte(`{"event":"PostToolUse","hooks":[{"type":"command","command":"./lint.sh"}]}`), 0644)

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./lint.sh"}]}`)
	item := catalog.ContentItem{Name: "test-relative", Path: itemDir}

	destDir := t.TempDir()
	result, _, err := resolveHookScripts(matcherGroup, item, destDir)
	if err != nil {
		t.Fatal(err)
	}

	cmd := gjson.GetBytes(result, "hooks.0.command").String()
	if cmd != filepath.Join(destDir, "lint.sh") {
		t.Errorf("expected rewritten path, got %q", cmd)
	}

	// Verify the script was copied
	if _, err := os.Stat(cmd); err != nil {
		t.Errorf("copied script not found at %s: %v", cmd, err)
	}
}

func TestResolveHookScripts_ScriptWithArgs(t *testing.T) {
	itemDir := t.TempDir()
	os.WriteFile(filepath.Join(itemDir, "check.sh"), []byte("#!/bin/bash"), 0755)

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./check.sh --strict --verbose"}]}`)
	item := catalog.ContentItem{Name: "test-args", Path: itemDir}

	result, _, err := resolveHookScripts(matcherGroup, item, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	cmd := gjson.GetBytes(result, "hooks.0.command").String()
	if !strings.Contains(cmd, "check.sh") || !strings.Contains(cmd, "--strict --verbose") {
		t.Errorf("expected rewritten path with args preserved, got %q", cmd)
	}
}

func TestResolveHookScripts_MissingScript(t *testing.T) {
	itemDir := t.TempDir()
	// No script file exists

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./nonexistent.sh"}]}`)
	item := catalog.ContentItem{Name: "test-missing", Path: itemDir}

	result, _, err := resolveHookScripts(matcherGroup, item, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// Command should be unchanged when script doesn't exist
	cmd := gjson.GetBytes(result, "hooks.0.command").String()
	if cmd != "./nonexistent.sh" {
		t.Errorf("command should be unchanged for missing script, got %q", cmd)
	}
}

// A script may source a helper beside it, so the whole item is copied with
// its layout and permissions, not only the script the command names.
func TestResolveHookScripts_CopiesTheWholeItem(t *testing.T) {
	itemDir := t.TempDir()
	os.MkdirAll(filepath.Join(itemDir, "lib"), 0755)
	os.WriteFile(filepath.Join(itemDir, "lint.sh"), []byte("#!/bin/bash\n. ./lib/common.sh"), 0644)
	os.WriteFile(filepath.Join(itemDir, "lib", "common.sh"), []byte("x=1"), 0640)

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./lint.sh"}]}`)
	destDir := t.TempDir()
	if _, _, err := resolveHookScripts(matcherGroup, catalog.ContentItem{Name: "whole", Path: itemDir}, destDir); err != nil {
		t.Fatal(err)
	}

	if fi, err := os.Stat(filepath.Join(destDir, "lib", "common.sh")); err != nil || fi.Mode().Perm() != 0640 {
		t.Errorf("helper not copied with its mode: %v %v", fi, err)
	}
	if fi, err := os.Stat(filepath.Join(destDir, "lint.sh")); err != nil || fi.Mode().Perm() != 0700 {
		t.Errorf("script not made executable: %v %v", fi, err)
	}
}

func TestResolveHookScripts_QuotesAPathWithASpace(t *testing.T) {
	itemDir := t.TempDir()
	os.WriteFile(filepath.Join(itemDir, "lint.sh"), []byte("#!/bin/bash"), 0755)

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./lint.sh --strict"}]}`)
	destDir := filepath.Join(t.TempDir(), "my hooks")
	result, _, err := resolveHookScripts(matcherGroup, catalog.ContentItem{Name: "spaced", Path: itemDir}, destDir)
	if err != nil {
		t.Fatal(err)
	}

	want := "'" + filepath.Join(destDir, "lint.sh") + "' --strict"
	if cmd := gjson.GetBytes(result, "hooks.0.command").String(); cmd != want {
		t.Errorf("command = %q, want %q", cmd, want)
	}
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/home/u/.syllago/hooks/a-b_c.sh": "/home/u/.syllago/hooks/a-b_c.sh",
		"/home/my home/lint.sh":           "'/home/my home/lint.sh'",
		"/tmp/it's/lint.sh":               `'/tmp/it'\''s/lint.sh'`,
		"/tmp/$(id)/lint.sh":              "'/tmp/$(id)/lint.sh'",
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

// A registry index can name an item anything, and the name becomes the
// scripts directory, so a name that is not one path element is refused.
func TestPlaceHook_RefusesANameThatIsNotOnePathElement(t *testing.T) {
	for _, name := range []string{"", ".", "..", "a/b", "../escape"} {
		_, err := PlaceHook(catalog.ContentItem{Name: name}, converter.Hook{}, provider.Provider{}, "", "", "", nil, "", ScanOptions{})
		if err == nil || !strings.Contains(err.Error(), "not a valid directory name") {
			t.Errorf("name %q: got %v, want a refusal", name, err)
		}
	}
}

// A content root reached through a symlink resolves to the same place as
// its scripts, so a bundled script is not mistaken for one outside it.
func TestResolveHookScripts_ItemReachedThroughASymlink(t *testing.T) {
	realDir := t.TempDir()
	os.WriteFile(filepath.Join(realDir, "lint.sh"), []byte("#!/bin/bash"), 0755)
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./lint.sh"}]}`)
	destDir := t.TempDir()
	if _, _, err := resolveHookScripts(matcherGroup, catalog.ContentItem{Name: "linked", Path: link}, destDir); err != nil {
		t.Fatalf("resolveHookScripts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "lint.sh")); err != nil {
		t.Errorf("script not copied: %v", err)
	}
}

// Only a legacy record for the same provider means the hook is installed:
// the same name installed for another provider leaves this one to place.
func TestHookAtLegacyRoot_MatchesTheProvider(t *testing.T) {
	legacyRoot := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = legacyRoot
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })
	if err := SaveInstalled(legacyRoot, &Installed{Hooks: []InstalledHook{{Name: "lint", Event: "BeforeTool", Provider: "gemini-cli"}}}); err != nil {
		t.Fatal(err)
	}

	projectRoot := t.TempDir()
	if event, ok := HookAtLegacyRoot(projectRoot, "lint", "gemini-cli"); !ok || event != "BeforeTool" {
		t.Errorf("gemini-cli: got %q, %v; want BeforeTool, true", event, ok)
	}
	if _, ok := HookAtLegacyRoot(projectRoot, "lint", "claude-code"); ok {
		t.Error("claude-code matched a gemini-cli record")
	}
}
