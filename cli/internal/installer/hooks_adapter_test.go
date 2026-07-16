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

// writeCanonicalHookItem writes a single-hook canonical manifest into a fresh
// project root and returns the item plus the project root.
func writeCanonicalHookItem(t *testing.T, name, event, matcher, command string) (catalog.ContentItem, string) {
	t.Helper()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	hookDir := filepath.Join(projectRoot, "hooks", name)
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"name":"` + name +
		`","event":"` + event + `","matcher":"` + matcher +
		`","handler":{"type":"command","command":"` + command + `"}}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	return catalog.ContentItem{Name: name, Type: catalog.Hooks, Path: hookDir}, projectRoot
}

// TestInstallHook_Adapter_SharedJSON_RoundTrip installs a canonical hook into
// each shared-JSON provider's native file via its converter.HookAdapter, decodes
// it back through the same adapter to prove it landed in the provider's real
// format, then exercises status + uninstall + re-install.
func TestInstallHook_Adapter_SharedJSON_RoundTrip(t *testing.T) {
	providers := []provider.Provider{
		provider.ClaudeCode,
		provider.Cursor,
		provider.GeminiCLI,
		provider.FactoryDroid,
		provider.Crush,
	}

	for _, prov := range providers {
		t.Run(prov.Slug, func(t *testing.T) {
			item, projectRoot := writeCanonicalHookItem(t, "guard", "before_tool_execute", "shell", "echo lint")

			configDir := t.TempDir()
			settingsPath := filepath.Join(configDir, "config.json")
			os.WriteFile(settingsPath, []byte(`{}`), 0644)
			overrideHookSettingsPath(t, settingsPath)

			if _, err := installHook(item, prov, projectRoot); err != nil {
				t.Fatalf("installHook: %v", err)
			}

			adapter := converter.AdapterFor(prov.Slug)
			if adapter == nil {
				t.Fatalf("no adapter for %s", prov.Slug)
			}

			data, _ := os.ReadFile(settingsPath)
			decoded, err := adapter.Decode(data)
			if err != nil {
				t.Fatalf("decode written file: %v", err)
			}
			if !hasCommand(decoded, "echo lint") {
				t.Fatalf("decoded hooks missing command %q, got: %s", "echo lint", data)
			}

			if status := checkHookStatus(item, prov, projectRoot); status != StatusInstalled {
				t.Errorf("status after install: got %v, want Installed", status)
			}

			if _, err := uninstallHook(item, prov, projectRoot); err != nil {
				t.Fatalf("uninstallHook: %v", err)
			}
			data, _ = os.ReadFile(settingsPath)
			decoded, _ = adapter.Decode(data)
			if hasCommand(decoded, "echo lint") {
				t.Errorf("hook still present after uninstall: %s", data)
			}
			if status := checkHookStatus(item, prov, projectRoot); status != StatusNotInstalled {
				t.Errorf("status after uninstall: got %v, want NotInstalled", status)
			}

			// Re-install must succeed (round-trip): the uninstall cleared the
			// installed.json record so the dedup check no longer trips.
			if _, err := installHook(item, prov, projectRoot); err != nil {
				t.Fatalf("re-install: %v", err)
			}
		})
	}
}

// TestInstallHook_Adapter_PreservesSiblings proves the shared-JSON merge writes
// only the `hooks` key, leaving unrelated config keys untouched on both install
// and uninstall.
func TestInstallHook_Adapter_PreservesSiblings(t *testing.T) {
	tests := []struct {
		prov      provider.Provider
		seed      string
		siblingOK func(data []byte) bool
	}{
		{
			prov: provider.ClaudeCode,
			seed: `{"permissions":{"allow":["Bash"]}}`,
			siblingOK: func(data []byte) bool {
				return gjson.GetBytes(data, "permissions.allow.0").String() == "Bash"
			},
		},
		{
			prov: provider.Crush,
			seed: `{"mcp":{"existing":{"command":"keep-me"}}}`,
			siblingOK: func(data []byte) bool {
				return gjson.GetBytes(data, "mcp.existing.command").String() == "keep-me"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.prov.Slug, func(t *testing.T) {
			item, projectRoot := writeCanonicalHookItem(t, "guard", "before_tool_execute", "shell", "echo lint")

			settingsPath := filepath.Join(t.TempDir(), "config.json")
			os.WriteFile(settingsPath, []byte(tt.seed), 0644)
			overrideHookSettingsPath(t, settingsPath)

			if _, err := installHook(item, tt.prov, projectRoot); err != nil {
				t.Fatalf("installHook: %v", err)
			}
			data, _ := os.ReadFile(settingsPath)
			if !tt.siblingOK(data) {
				t.Errorf("sibling key clobbered by install: %s", data)
			}

			if _, err := uninstallHook(item, tt.prov, projectRoot); err != nil {
				t.Fatalf("uninstallHook: %v", err)
			}
			data, _ = os.ReadFile(settingsPath)
			if !tt.siblingOK(data) {
				t.Errorf("sibling key clobbered by uninstall: %s", data)
			}
		})
	}
}

// TestInstallHook_Windsurf_DeferredToPhase1b: windsurf is deferred to Phase 1b.
// Its adapter fans one before_tool_execute hook out to four split-events and
// only merges them back when each has exactly one entry, so a hook's
// post-round-trip identity is not stable once a second windsurf hook exists —
// uninstall/status/orphans can't reliably match it. Until Phase 1b adds a
// stable per-entry identity, installing a windsurf hook must reject and write
// nothing.
func TestInstallHook_Windsurf_DeferredToPhase1b(t *testing.T) {
	item, projectRoot := writeCanonicalHookItem(t, "guard", "before_tool_execute", "shell", "echo hi")

	settingsPath := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(settingsPath, []byte(`{}`), 0644)
	overrideHookSettingsPath(t, settingsPath)

	_, err := installHook(item, provider.Windsurf, projectRoot)
	if err == nil {
		t.Fatal("expected error installing hook to windsurf (Phase 1b)")
	}
	if !strings.Contains(err.Error(), "Phase 1b") {
		t.Errorf("error should reference Phase 1b, got: %v", err)
	}
	data, _ := os.ReadFile(settingsPath)
	if gjson.GetBytes(data, "hooks").Exists() {
		t.Errorf("no hook should have been written, got: %s", data)
	}
}

// TestWriteHookFile_DedicatedMode covers the dedicated-file write branch
// directly. That branch is Phase 1b infrastructure (no Phase 1 provider routes
// to it), so this keeps it exercised without a routed provider: it must write
// the encoded content verbatim as the whole file.
func TestWriteHookFile_DedicatedMode(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "hooks.json")
	encoded := []byte(`{"hooks":{"pre_run_command":[{"command":"echo hi"}]}}`)

	if err := writeHookFile(hookStorageDedicatedFile, path, encoded); err != nil {
		t.Fatalf("writeHookFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != string(encoded) {
		t.Errorf("dedicated write should be verbatim:\n got  %s\n want %s", got, encoded)
	}
}

// TestInstallHook_Adapter_RejectsNoEncoder: providers with no HookAdapter (amp,
// codex) are rejected with an honest error and nothing is written.
func TestInstallHook_Adapter_RejectsNoEncoder(t *testing.T) {
	item, projectRoot := writeCanonicalHookItem(t, "guard", "before_tool_execute", "shell", "echo hi")

	settingsPath := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(settingsPath, []byte(`{}`), 0644)
	overrideHookSettingsPath(t, settingsPath)

	_, err := installHook(item, provider.Amp, projectRoot)
	if err == nil {
		t.Fatal("expected error installing hook to amp (no adapter)")
	}
	if !strings.Contains(err.Error(), "no encoder") {
		t.Errorf("error should mention missing encoder, got: %v", err)
	}
	data, _ := os.ReadFile(settingsPath)
	if gjson.GetBytes(data, "hooks").Exists() {
		t.Errorf("no hook should have been written, got: %s", data)
	}
}

// TestInstallHook_Adapter_RejectsDirectoryProvider: directory-scoped hook
// providers (copilot-cli, kiro, pi) are rejected pending ADR-0020 Phase 1b.
func TestInstallHook_Adapter_RejectsDirectoryProvider(t *testing.T) {
	item, projectRoot := writeCanonicalHookItem(t, "guard", "before_tool_execute", "shell", "echo hi")

	settingsPath := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(settingsPath, []byte(`{}`), 0644)
	overrideHookSettingsPath(t, settingsPath)

	_, err := installHook(item, provider.CopilotCLI, projectRoot)
	if err == nil {
		t.Fatal("expected error installing hook to copilot-cli (Phase 1b)")
	}
	if !strings.Contains(err.Error(), "Phase 1b") {
		t.Errorf("error should reference Phase 1b, got: %v", err)
	}
	data, _ := os.ReadFile(settingsPath)
	if gjson.GetBytes(data, "hooks").Exists() {
		t.Errorf("no hook should have been written, got: %s", data)
	}
}

// hasCommand reports whether any decoded canonical hook has the given command.
func hasCommand(ch *converter.CanonicalHooks, cmd string) bool {
	if ch == nil {
		return false
	}
	for _, h := range ch.Hooks {
		if h.Handler.Command == cmd {
			return true
		}
	}
	return false
}
