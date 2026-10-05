package installer

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// conversionWarnings returns the messages of the conversion warnings in
// notices.
func conversionWarnings(notices []Notice) []string {
	var msgs []string
	for _, n := range notices {
		if n.Kind == NoticeConversionWarning {
			msgs = append(msgs, n.Message)
		}
	}
	return msgs
}

// TestInstallHook_ReportsWhatTheTargetLoses: a hook the target's format
// changes comes back with the encoder's warnings, named for the item, so
// install shows them without converting the hook a second time.
func TestInstallHook_ReportsWhatTheTargetLoses(t *testing.T) {
	tests := []struct {
		prov provider.Provider
		want string // "" means no conversion warning
	}{
		{provider.Devin, "guard: devin PreToolUse hooks always have veto power"},
		{provider.ClaudeCode, ""},
	}
	for _, tt := range tests {
		t.Run(tt.prov.Slug, func(t *testing.T) {
			item, projectRoot := writeCanonicalHookItem(t, "guard", "before_tool_execute", "shell", "echo hi")
			settingsPath := filepath.Join(t.TempDir(), "settings.json")
			os.WriteFile(settingsPath, []byte(`{}`), 0644)
			overrideHookSettingsPath(t, settingsPath)

			placement, err := installHook(item, tt.prov, projectRoot, ScanOptions{})
			if err != nil {
				t.Fatalf("installHook: %v", err)
			}
			got := conversionWarnings(placement.Notices)
			if tt.want == "" {
				if len(got) != 0 {
					t.Errorf("expected no conversion warnings, got %q", got)
				}
				return
			}
			found := false
			for _, msg := range got {
				found = found || strings.HasPrefix(msg, tt.want)
			}
			if !found {
				t.Errorf("expected a warning starting %q, got %q", tt.want, got)
			}
		})
	}
}

// TestInstall_PlacedItemReportsWhatTheTargetLoses: a skill placed as it is
// still loses its hooks on a provider that reads hooks only from its own
// settings, so the placement says so.
func TestInstall_PlacedItemReportsWhatTheTargetLoses(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "skills", "fmt")
	os.MkdirAll(skillDir, 0755)
	skill := "---\nname: fmt\ndescription: Formats code\nhooks:\n  PreToolUse:\n    - matcher: Bash\n      hooks:\n        - type: command\n          command: echo hi\n---\n\nFormat it.\n"
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0644)
	item := catalog.ContentItem{Name: "fmt", Type: catalog.Skills, Path: skillDir}

	tests := []struct {
		prov provider.Provider
		want string // "" means no conversion warning
	}{
		{provider.GeminiCLI, `fmt: skill "fmt" has hooks requiring separate configuration`},
		{provider.ClaudeCode, ""},
	}
	for _, tt := range tests {
		t.Run(tt.prov.Slug, func(t *testing.T) {
			placement, err := Install(item, tt.prov, t.TempDir(), MethodSymlink, t.TempDir(), ScanOptions{})
			if err != nil {
				t.Fatalf("Install: %v", err)
			}
			if placement.Mechanism != MechanismSymlink {
				t.Fatalf("mechanism: got %v, want symlink", placement.Mechanism)
			}
			got := conversionWarnings(placement.Notices)
			if tt.want == "" {
				if len(got) != 0 {
					t.Errorf("expected no conversion warnings, got %q", got)
				}
				return
			}
			if len(got) == 0 || got[0] != tt.want+":" {
				t.Errorf("first warning: got %q, want %q", got, tt.want+":")
			}
		})
	}
}

// TestInstall_ItemPlacedForItsOwnProviderLosesNothing: an item placed for
// the provider it came from is already in that provider's terms, so a
// warning rendering it would give does not apply.
func TestInstall_ItemPlacedForItsOwnProviderLosesNothing(t *testing.T) {
	home := t.TempDir()
	prov := provider.Provider{
		Name: "Cline",
		Slug: "cline",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			return filepath.Join(homeDir, ".cline", "commands")
		},
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Commands },
	}
	cmdDir := filepath.Join(t.TempDir(), "commands", "cline", "deploy")
	os.MkdirAll(cmdDir, 0755)
	os.WriteFile(filepath.Join(cmdDir, "command.md"), []byte("---\ndescription: Deploy\n---\n\nDeploy $ARGUMENTS.\n"), 0644)
	item := catalog.ContentItem{Name: "deploy", Type: catalog.Commands, Provider: "cline", Path: cmdDir}

	placement, err := Install(item, prov, t.TempDir(), MethodSymlink, home, ScanOptions{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got := conversionWarnings(placement.Notices); len(got) != 0 {
		t.Errorf("expected no conversion warnings, got %q", got)
	}
}

// TestInstall_ItemAddedFromTheTargetLosesNothing: a universal item records
// the provider it was added from in its metadata rather than its
// directory, and placing it back there loses nothing.
func TestInstall_ItemAddedFromTheTargetLosesNothing(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "skills", "fmt")
	os.MkdirAll(skillDir, 0755)
	skill := "---\nname: fmt\ndescription: Formats code\nhooks:\n  PreToolUse:\n    - matcher: Bash\n      hooks:\n        - type: command\n          command: echo hi\n---\n\nFormat it.\n"
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0644)
	item := catalog.ContentItem{Name: "fmt", Type: catalog.Skills, Path: skillDir, Meta: &metadata.Meta{SourceProvider: "gemini-cli"}}

	placement, err := Install(item, provider.GeminiCLI, t.TempDir(), MethodSymlink, t.TempDir(), ScanOptions{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got := conversionWarnings(placement.Notices); len(got) != 0 {
		t.Errorf("expected no conversion warnings, got %q", got)
	}
}

// TestInstallMCP_ReportsFieldsItDoesNotWrite: the merge writes a server's
// type, command, args, url and env for every provider, the one it came from
// included, so install names the stored fields it leaves out.
func TestInstallMCP_ReportsFieldsItDoesNotWrite(t *testing.T) {
	isolateLegacyRoot(t)
	itemDir := filepath.Join(t.TempDir(), "mcp", "gh")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{"mcpServers":{"gh":{"type":"http","url":"https://x","headers":{"Authorization":"t"},"alwaysAllow":["list"]}}}`), 0644)
	want := []string{`gh: server "gh": alwaysAllow, headers not installed (syllago writes only type, command, args, url, env)`}

	for _, prov := range []provider.Provider{provider.Cursor, provider.Cline} {
		t.Run(prov.Slug, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "mcp.json")
			overrideMCPConfigPaths(t, map[string]string{prov.Slug: cfgPath})
			item := catalog.ContentItem{Name: "gh", Type: catalog.MCP, Path: itemDir, Meta: &metadata.Meta{SourceProvider: "cline"}}

			placement, err := installMCP(item, prov, t.TempDir())
			if err != nil {
				t.Fatalf("installMCP: %v", err)
			}
			if got := conversionWarnings(placement.Notices); !slices.Equal(got, want) {
				t.Errorf("warnings: got %q, want %q", got, want)
			}
			written, _ := os.ReadFile(cfgPath)
			if strings.Contains(string(written), "headers") || strings.Contains(string(written), "alwaysAllow") {
				t.Errorf("a field the warning names was written: %s", written)
			}
		})
	}
}

// TestInstallMCP_ReportsOnlyTheInstalledServer: an item for one server of a
// shared config installs that server alone, so another server's fields are
// not its warnings, and a server with nothing left out has none.
func TestInstallMCP_ReportsOnlyTheInstalledServer(t *testing.T) {
	isolateLegacyRoot(t)
	itemDir := filepath.Join(t.TempDir(), "mcp", "shared")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{"mcpServers":{"a":{"command":"a-mcp","args":["-v"]},"b":{"command":"b-mcp","autoApprove":["y"]}}}`), 0644)

	tests := []struct {
		server string
		want   []string
	}{
		{"a", nil},
		{"b", []string{`b: server "b": autoApprove not installed (syllago writes only type, command, args, url, env)`}},
	}
	for _, tt := range tests {
		t.Run(tt.server, func(t *testing.T) {
			overrideMCPConfigPaths(t, map[string]string{"cursor": filepath.Join(t.TempDir(), "cursor-mcp.json")})
			item := catalog.ContentItem{Name: tt.server, Type: catalog.MCP, Path: itemDir, ServerKey: tt.server}

			placement, err := installMCP(item, provider.Cursor, t.TempDir())
			if err != nil {
				t.Fatalf("installMCP: %v", err)
			}
			if got := conversionWarnings(placement.Notices); !slices.Equal(got, tt.want) {
				t.Errorf("warnings: got %q, want %q", got, tt.want)
			}
		})
	}
}
