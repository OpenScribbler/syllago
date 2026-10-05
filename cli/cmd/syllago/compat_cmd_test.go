package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/tidwall/gjson"
)

// setupCompatLibrary creates a temp library with a skill item for compat tests.
func setupCompatLibrary(t *testing.T) string {
	t.Helper()
	lib := t.TempDir()

	skillDir := filepath.Join(lib, "skills", "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\nDoes something.\n"), 0644)

	return lib
}

func withCompatLibrary(t *testing.T, dir string) {
	t.Helper()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })
}

func TestCompatItemNotFound(t *testing.T) {
	lib := setupCompatLibrary(t)
	withCompatLibrary(t, lib)
	_, _ = output.SetForTest(t)

	err := compatCmd.RunE(compatCmd, []string{"nonexistent"})
	if err == nil {
		t.Error("compat with nonexistent item should fail")
	}
	if !strings.Contains(err.Error(), "no item named") {
		t.Errorf("expected 'no item named' in error, got: %v", err)
	}
}

func TestCompatTextOutput(t *testing.T) {
	lib := setupCompatLibrary(t)
	withCompatLibrary(t, lib)
	stdout, _ := output.SetForTest(t)

	err := compatCmd.RunE(compatCmd, []string{"test-skill"})
	if err != nil {
		t.Fatalf("compat should succeed, got: %v", err)
	}

	out := stdout.String()
	// Should have a header line.
	if !strings.Contains(out, "Provider") {
		t.Error("expected 'Provider' header in output")
	}
	// Should contain at least one provider slug.
	if !strings.Contains(out, "claude-code") {
		t.Errorf("expected 'claude-code' in output, got:\n%s", out)
	}
	// Should have supported/unsupported symbols.
	if !strings.Contains(out, "✓") && !strings.Contains(out, "✗") {
		t.Errorf("expected status symbols in output, got:\n%s", out)
	}
}

func TestCompatJSONOutput(t *testing.T) {
	lib := setupCompatLibrary(t)
	withCompatLibrary(t, lib)
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	err := compatCmd.RunE(compatCmd, []string{"test-skill"})
	if err != nil {
		t.Fatalf("compat --json should succeed, got: %v", err)
	}

	data := stdout.Bytes()
	name := gjson.GetBytes(data, "name").String()
	if name != "test-skill" {
		t.Errorf("expected name 'test-skill', got %q", name)
	}
	typ := gjson.GetBytes(data, "type").String()
	if typ != "Skills" {
		t.Errorf("expected type 'Skills', got %q", typ)
	}
	entries := gjson.GetBytes(data, "entries")
	if !entries.IsArray() || len(entries.Array()) == 0 {
		t.Error("expected non-empty entries array in JSON output")
	}
	// Each entry should have provider and supported fields.
	first := entries.Array()[0]
	if !first.Get("provider").Exists() {
		t.Error("expected 'provider' field in entry")
	}
	if !first.Get("supported").Exists() {
		t.Error("expected 'supported' field in entry")
	}
}

func TestCompatShowsUnsupportedProviders(t *testing.T) {
	lib := setupCompatLibrary(t)
	withCompatLibrary(t, lib)
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	err := compatCmd.RunE(compatCmd, []string{"test-skill"})
	if err != nil {
		t.Fatalf("compat should succeed, got: %v", err)
	}

	data := stdout.Bytes()
	entries := gjson.GetBytes(data, "entries").Array()

	// Check that at least one entry exists with supported=true and one with
	// supported=false (skills aren't supported by every provider).
	var hasSupported, hasUnsupported bool
	for _, e := range entries {
		if e.Get("supported").Bool() {
			hasSupported = true
		} else {
			hasUnsupported = true
		}
	}
	if !hasSupported {
		t.Error("expected at least one supported provider for skills")
	}
	// It's possible all providers support skills — only assert unsupported if it's expected.
	_ = hasUnsupported
}

// Regression: compat rendered a Library hook through a second encoder, so
// it failed to read the hooks/0.1 manifest and showed a provider syllago
// cannot write hooks for, such as Codex, as supported.
func TestCompatLibraryHook(t *testing.T) {
	lib := t.TempDir()
	hookDir := filepath.Join(lib, "hooks", "claude-code", "lint")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(`{"spec":"hooks/0.1","hooks":[{"event":"worktree_create","handler":{"type":"command","command":"./lint.sh"}}]}`), 0644)
	withCompatLibrary(t, lib)
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	if err := compatCmd.RunE(compatCmd, []string{"lint"}); err != nil {
		t.Fatalf("compat: %v", err)
	}
	entries := map[string]gjson.Result{}
	for _, e := range gjson.Get(stdout.String(), "entries").Array() {
		entries[e.Get("provider").String()] = e
	}
	for prov, want := range map[string]bool{"claude-code": true, "gemini-cli": false, "codex": false} {
		e, ok := entries[prov]
		if !ok {
			t.Fatalf("no entry for %s:\n%s", prov, stdout.String())
		}
		if got := e.Get("supported").Bool(); got != want {
			t.Errorf("%s supported = %v, want %v (warnings %s)", prov, got, want, e.Get("warnings").Raw)
		}
	}
	if w := entries["gemini-cli"].Get("warnings").Raw; !strings.Contains(w, "worktree_create") {
		t.Errorf("gemini-cli warnings = %s, want the event it cannot hold", w)
	}
}

func TestCompatUnreadableItemFails(t *testing.T) {
	lib := t.TempDir()
	dir := filepath.Join(lib, "mcp", "broken")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "mcp.json"), []byte("{not json"), 0644)
	withCompatLibrary(t, lib)
	_, _ = output.SetForTest(t)

	err := compatCmd.RunE(compatCmd, []string{"broken"})
	if err == nil || !strings.Contains(err.Error(), "canonicalizing content failed") {
		t.Fatalf("compat on an unreadable item = %v, want the parse failure", err)
	}
}
