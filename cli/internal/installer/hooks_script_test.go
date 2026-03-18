package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/tidwall/gjson"
)

func TestResolveHookScripts_InlineCommand(t *testing.T) {
	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "echo lint"}]}`)
	item := catalog.ContentItem{Name: "test-hook", Path: t.TempDir()}

	result, err := resolveHookScripts(matcherGroup, item, t.TempDir())
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

	result, err := resolveHookScripts(matcherGroup, item, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	cmd := gjson.GetBytes(result, "hooks.0.command").String()
	if !strings.Contains(cmd, ".syllago/hooks/test-relative/lint.sh") {
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

	result, err := resolveHookScripts(matcherGroup, item, t.TempDir())
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

	result, err := resolveHookScripts(matcherGroup, item, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// Command should be unchanged when script doesn't exist
	cmd := gjson.GetBytes(result, "hooks.0.command").String()
	if cmd != "./nonexistent.sh" {
		t.Errorf("command should be unchanged for missing script, got %q", cmd)
	}
}
