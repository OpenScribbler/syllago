package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
)

// writeEventHookItem writes a canonical hook.json with the given event into a
// fresh project root and returns the item plus the project root.
func writeEventHookItem(t *testing.T, event string) (catalog.ContentItem, string) {
	t.Helper()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	hookDir := filepath.Join(projectRoot, "hooks", "event-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := fmt.Sprintf(`{"spec":"hooks/0.1","hooks":[{"event":%q,"handler":{"type":"command","command":"echo hi"}}]}`, event)
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	return catalog.ContentItem{
		Name: "event-hook",
		Type: catalog.Hooks,
		Path: hookDir,
	}, projectRoot
}

// TestInstallHook_RejectsUnsupportedEvent: a hook whose event the target
// provider has no settings key for must fail the install instead of merging
// dead config under a key the provider never reads (syllago-xqlc1). Covers
// both doors into the bug: a canonical event with no mapping for the
// provider, and another provider's native event name.
func TestInstallHook_RejectsUnsupportedEvent(t *testing.T) {
	tests := []struct {
		name  string
		event string
		prov  provider.Provider
	}{
		// Both cases use routed (Phase 1) providers so the reject comes from the
		// event-support gate, not the Phase-1b storage-model rejection. windsurf
		// is not used here: it is deferred to Phase 1b, so it rejects before the
		// event gate (see TestInstallHook_Windsurf_DeferredToPhase1b).
		//
		// "PostToolUse" is claude-code's native name for after_tool_execute —
		// crush supports only before_tool_execute, so it's a foreign/unreadable
		// event for crush.
		{"another provider's native name", "PostToolUse", provider.Crush},
		{"canonical event unmapped for crush", "session_end", provider.Crush},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, projectRoot := writeEventHookItem(t, tt.event)

			settingsPath := filepath.Join(t.TempDir(), "settings.json")
			os.WriteFile(settingsPath, []byte(`{}`), 0644)
			overrideHookSettingsPath(t, settingsPath)

			_, err := installHook(item, tt.prov, projectRoot)
			if err == nil {
				t.Fatalf("expected error installing %q hook to %s", tt.event, tt.prov.Slug)
			}
			if !strings.Contains(err.Error(), "does not support hook event") {
				t.Errorf("error should name the unsupported event, got: %v", err)
			}
			data, _ := os.ReadFile(settingsPath)
			if gjson.GetBytes(data, "hooks").Exists() {
				t.Errorf("no hook should have been written, got: %s", data)
			}
		})
	}
}

// TestInstallHook_OwnNativeEventPassesThrough: a provider's OWN native event
// name is not rejected — it canonicalizes back to a supported event and
// installs. crush's native "PreToolUse" maps to canonical before_tool_execute.
func TestInstallHook_OwnNativeEventPassesThrough(t *testing.T) {
	item, projectRoot := writeEventHookItem(t, "PreToolUse")

	settingsPath := filepath.Join(t.TempDir(), "crush.json")
	os.WriteFile(settingsPath, []byte(`{}`), 0644)
	overrideHookSettingsPath(t, settingsPath)

	if _, err := installHook(item, provider.Crush, projectRoot); err != nil {
		t.Fatalf("installHook: %v", err)
	}
	data, _ := os.ReadFile(settingsPath)
	if !gjson.GetBytes(data, "hooks.PreToolUse.0").Exists() {
		t.Errorf("expected hooks.PreToolUse.0 in crush.json, got: %s", data)
	}
}
