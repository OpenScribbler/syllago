package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
)

// writeHookItem writes a canonical hook.json with the given matcher into a
// fresh project root and returns the item plus the project root.
func writeHookItem(t *testing.T, matcher string) (catalog.ContentItem, string) {
	t.Helper()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	hookDir := filepath.Join(projectRoot, "hooks", "matcher-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"name":"guard","event":"before_tool_execute","matcher":"` + matcher + `","handler":{"type":"command","command":"echo guard"}}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	return catalog.ContentItem{
		Name: "matcher-hook",
		Type: catalog.Hooks,
		Path: hookDir,
	}, projectRoot
}

// TestInstallHook_TranslatesMatcher verifies installHook translates canonical
// matcher tool names to each provider's native names before the JSON merge
// (syllago-9qgwt). Library hook.json stores canonical matchers ("shell");
// providers match their hook regexes against native tool names ("Bash"), so
// an untranslated matcher silently never fires. Crush is covered separately
// in hooks_crush_test.go via its flat-entry path.
//
// The full install -> status -> uninstall cycle runs per provider to verify
// hash matching stays consistent with the translated group.
func TestInstallHook_TranslatesMatcher(t *testing.T) {
	tests := []struct {
		prov        provider.Provider
		eventKey    string // provider-native settings key the hook lands under
		wantMatcher string // native translation of canonical "shell"
	}{
		{provider.ClaudeCode, "PreToolUse", "Bash"},
		{provider.GeminiCLI, "BeforeTool", "run_shell_command"},
		{provider.Cursor, "PreToolUse", "run_terminal_cmd"},
		// Windsurf is absent: it has no before_tool_execute mapping, so the
		// install is rejected outright — see
		// TestInstallHook_RejectsUnsupportedEvent (syllago-xqlc1).
	}

	for _, tt := range tests {
		t.Run(tt.prov.Slug, func(t *testing.T) {
			item, projectRoot := writeHookItem(t, "shell")

			configDir := t.TempDir()
			settingsPath := filepath.Join(configDir, "settings.json")
			os.WriteFile(settingsPath, []byte(`{}`), 0644)
			overrideHookSettingsPath(t, settingsPath)

			if _, err := installHook(item, tt.prov, projectRoot); err != nil {
				t.Fatalf("installHook: %v", err)
			}

			data, _ := os.ReadFile(settingsPath)
			entry := gjson.GetBytes(data, "hooks."+tt.eventKey+".0")
			if !entry.Exists() {
				t.Fatalf("expected hooks.%s.0 in settings, got: %s", tt.eventKey, data)
			}
			if got := entry.Get("matcher").String(); got != tt.wantMatcher {
				t.Errorf("matcher: got %q, want %q (canonical shell -> %s native)", got, tt.wantMatcher, tt.prov.Slug)
			}
			if got := entry.Get("hooks.0.command").String(); got != "echo guard" {
				t.Errorf("command: got %q, want 'echo guard'", got)
			}

			// Hash matching is computed over the merged (translated) group —
			// status and uninstall must still line up.
			if status := checkHookStatus(item, tt.prov, projectRoot); status != StatusInstalled {
				t.Errorf("expected StatusInstalled, got %v", status)
			}
			if _, err := uninstallHook(item, tt.prov, projectRoot); err != nil {
				t.Fatalf("uninstallHook: %v", err)
			}
			data, _ = os.ReadFile(settingsPath)
			if gjson.GetBytes(data, "hooks."+tt.eventKey+".0").Exists() {
				t.Errorf("hook entry should be removed, got: %s", data)
			}
		})
	}
}

// TestInstallHook_MatcherPatterns covers the matcher shapes TranslateMatcher
// must handle during install: regex alternations translate per component,
// wildcards and already-native names pass through unchanged.
func TestInstallHook_MatcherPatterns(t *testing.T) {
	tests := []struct {
		name        string
		matcher     string
		wantMatcher string
	}{
		{"alternation", "file_edit|file_write", "Edit|Write"},
		{"wildcard", ".*", ".*"},
		{"already native", "Bash", "Bash"},
		{"mcp pattern", "mcp__github__.*", "mcp__github__.*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, projectRoot := writeHookItem(t, tt.matcher)

			configDir := t.TempDir()
			settingsPath := filepath.Join(configDir, "settings.json")
			os.WriteFile(settingsPath, []byte(`{}`), 0644)
			overrideHookSettingsPath(t, settingsPath)

			if _, err := installHook(item, provider.ClaudeCode, projectRoot); err != nil {
				t.Fatalf("installHook: %v", err)
			}

			data, _ := os.ReadFile(settingsPath)
			got := gjson.GetBytes(data, "hooks.PreToolUse.0.matcher").String()
			if got != tt.wantMatcher {
				t.Errorf("matcher: got %q, want %q", got, tt.wantMatcher)
			}
		})
	}
}
