package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestCheckOrphanedMerges_NoOrphans(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Empty installed.json and no providers with settings
	inst := &Installed{}
	SaveInstalled(projectRoot, inst)

	orphans, err := CheckOrphanedMerges(projectRoot, nil)
	if err != nil {
		t.Fatalf("CheckOrphanedMerges: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected no orphans, got %d", len(orphans))
	}
}

func TestCheckOrphanedMerges_DetectsOrphanedHook(t *testing.T) {
	// Not parallel — uses HOME override via provider config
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create a claude-code config directory with two hooks in its native
	// settings.json — orphan detection now decodes via the provider's adapter.
	configDir := filepath.Join(home, ".syllago-test-orphan-"+filepath.Base(projectRoot))
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("MkdirAll config dir: %v", err)
	}

	settingsJSON := `{
  "hooks": {
    "PreToolUse": [
      {"matcher":"Bash","hooks":[{"type":"command","command":"echo tracked"}]},
      {"matcher":"Edit","hooks":[{"type":"command","command":"echo orphan"}]}
    ]
  }
}`
	if err := os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(settingsJSON), 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	prov := provider.Provider{
		Name:      "Claude Code",
		Slug:      "claude-code",
		ConfigDir: filepath.Base(configDir),
		Detected:  true,
	}

	// Track only the "echo tracked" hook, using the SAME canonical identity
	// (decode via the adapter, then hookIdentity) that install records.
	adapter := converter.AdapterFor("claude-code")
	decoded, err := adapter.Decode([]byte(settingsJSON))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var trackedHash string
	for _, hk := range decoded.Hooks {
		if hk.Handler.Command == "echo tracked" {
			trackedHash = hookIdentity(hk)
		}
	}
	if trackedHash == "" {
		t.Fatal("could not compute identity for tracked hook")
	}

	inst := &Installed{
		Hooks: []InstalledHook{
			{Name: "tracked-hook", Event: "PreToolUse", GroupHash: trackedHash, Command: "echo tracked"},
		},
	}
	if err := SaveInstalled(projectRoot, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	orphans, err := CheckOrphanedMerges(projectRoot, []provider.Provider{prov})
	if err != nil {
		t.Fatalf("CheckOrphanedMerges: %v", err)
	}

	// Should find exactly one orphan: the untracked "echo orphan" entry.
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d: %+v", len(orphans), orphans)
	}
	if orphans[0].Type != "hook" {
		t.Errorf("orphan type = %q, want hook", orphans[0].Type)
	}
	if orphans[0].Key != "PreToolUse" {
		t.Errorf("orphan key = %q, want PreToolUse", orphans[0].Key)
	}
}

func TestCheckOrphanedMerges_DirectoryProviderReadsOwnedFileOnly(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755); err != nil {
		t.Fatalf("MkdirAll .syllago: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	prov := provider.Pi
	prov.Detected = true
	adapter := converter.AdapterFor(prov.Slug)
	if adapter == nil {
		t.Fatalf("no adapter for %s", prov.Slug)
	}

	matcher, _ := json.Marshal("shell")
	encoded, err := adapter.Encode(&converter.CanonicalHooks{
		Spec: converter.SpecVersion,
		Hooks: []converter.CanonicalHook{
			{
				Name:     "tracked",
				Event:    "before_tool_execute",
				Matcher:  matcher,
				Blocking: true,
				Handler:  converter.HookHandler{Type: "command", Command: "echo tracked"},
			},
			{
				Name:     "orphan",
				Event:    "before_tool_execute",
				Matcher:  matcher,
				Blocking: true,
				Handler:  converter.HookHandler{Type: "command", Command: "echo orphan"},
			},
		},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	hookPath, err := HookConfigPath(prov, home)
	if err != nil {
		t.Fatalf("HookConfigPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(hookPath), 0755); err != nil {
		t.Fatalf("MkdirAll hook dir: %v", err)
	}
	if err := os.WriteFile(hookPath, encoded.Content, 0644); err != nil {
		t.Fatalf("write hook file: %v", err)
	}
	siblingPath := filepath.Join(filepath.Dir(hookPath), "user-extension.ts")
	const siblingContent = "not managed by syllago"
	if err := os.WriteFile(siblingPath, []byte(siblingContent), 0644); err != nil {
		t.Fatalf("write sibling: %v", err)
	}

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var trackedHash string
	for _, hk := range decoded.Hooks {
		if hk.Handler.Command == "echo tracked" {
			trackedHash = hookIdentity(hk)
		}
	}
	if trackedHash == "" {
		t.Fatal("could not compute identity for tracked hook")
	}
	if err := SaveInstalled(projectRoot, &Installed{
		Hooks: []InstalledHook{
			{Name: "tracked", Event: "tool_call", GroupHash: trackedHash, Command: "echo tracked"},
		},
	}); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	orphans, err := CheckOrphanedMerges(projectRoot, []provider.Provider{prov})
	if err != nil {
		t.Fatalf("CheckOrphanedMerges: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d: %+v", len(orphans), orphans)
	}
	if orphans[0].Provider != "pi" || orphans[0].Type != "hook" || orphans[0].Key != "tool_call" {
		t.Fatalf("unexpected orphan: %+v", orphans[0])
	}
	gotSibling, err := os.ReadFile(siblingPath)
	if err != nil {
		t.Fatalf("read sibling: %v", err)
	}
	if string(gotSibling) != siblingContent {
		t.Fatalf("sibling changed: got %q, want %q", gotSibling, siblingContent)
	}
}

func TestCheckOrphanedMerges_DetectsOrphanedMCP(t *testing.T) {
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".syllago-test-orphan-mcp-"+filepath.Base(projectRoot))
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("MkdirAll config dir: %v", err)
	}

	settingsJSON := `{
  "mcpServers": {
    "tracked-server": {"command": "node", "args": ["s.js"]},
    "orphan-server": {"command": "node", "args": ["evil.js"]}
  }
}`
	if err := os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(settingsJSON), 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	inst := &Installed{
		MCP: []InstalledMCP{
			{Name: "tracked", ServerKey: "tracked-server"},
		},
	}
	if err := SaveInstalled(projectRoot, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	prov := provider.Provider{
		Name:      "Test",
		Slug:      "test",
		ConfigDir: filepath.Base(configDir),
		Detected:  true,
	}

	orphans, err := CheckOrphanedMerges(projectRoot, []provider.Provider{prov})
	if err != nil {
		t.Fatalf("CheckOrphanedMerges: %v", err)
	}

	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].Type != "mcp" {
		t.Errorf("orphan type = %q, want mcp", orphans[0].Type)
	}
	if orphans[0].Key != "orphan-server" {
		t.Errorf("orphan key = %q, want orphan-server", orphans[0].Key)
	}
}

func TestCheckOrphanedMerges_SkipsUndetectedProviders(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	inst := &Installed{}
	SaveInstalled(projectRoot, inst)

	// Provider not detected — should be skipped entirely
	prov := provider.Provider{
		Name:      "Undetected",
		Slug:      "undetected",
		ConfigDir: ".undetected",
		Detected:  false,
	}

	orphans, err := CheckOrphanedMerges(projectRoot, []provider.Provider{prov})
	if err != nil {
		t.Fatalf("CheckOrphanedMerges: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected no orphans for undetected provider, got %d", len(orphans))
	}
}
