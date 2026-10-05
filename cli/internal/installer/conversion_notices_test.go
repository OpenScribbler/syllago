package installer

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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

// TestInstallMCP_ReportsWhatTheTargetLoses: an MCP server merged into a
// provider's config keeps fields the provider does not read, so the
// placement says which ones it ignores.
func TestInstallMCP_ReportsWhatTheTargetLoses(t *testing.T) {
	isolateLegacyRoot(t)
	dir := t.TempDir()
	overrideMCPConfigPaths(t, map[string]string{
		"cursor":      filepath.Join(dir, "cursor-mcp.json"),
		"claude-code": filepath.Join(dir, "claude.json"),
	})
	itemDir := filepath.Join(t.TempDir(), "mcp", "gh")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{"mcpServers":{"gh":{"command":"gh-mcp","autoApprove":["list"]}}}`), 0644)
	item := catalog.ContentItem{Name: "gh", Type: catalog.MCP, Path: itemDir}

	tests := []struct {
		prov provider.Provider
		want string // "" means no conversion warning
	}{
		{provider.Cursor, `gh: server "gh": autoApprove dropped (not documented by Cursor)`},
		{provider.ClaudeCode, ""},
	}
	for _, tt := range tests {
		t.Run(tt.prov.Slug, func(t *testing.T) {
			placement, err := installMCP(item, tt.prov, t.TempDir())
			if err != nil {
				t.Fatalf("installMCP: %v", err)
			}
			got := conversionWarnings(placement.Notices)
			if tt.want == "" {
				if len(got) != 0 {
					t.Errorf("expected no conversion warnings, got %q", got)
				}
				return
			}
			if !slices.Contains(got, tt.want) {
				t.Errorf("expected warning %q, got %q", tt.want, got)
			}
		})
	}
}

// TestInstallMCP_ReadsTheServerInItsSourceFormat: a server added from Cline
// keeps Cline's alwaysAllow, which is what Cursor would lose.
func TestInstallMCP_ReadsTheServerInItsSourceFormat(t *testing.T) {
	isolateLegacyRoot(t)
	overrideMCPConfigPaths(t, map[string]string{"cursor": filepath.Join(t.TempDir(), "cursor-mcp.json")})
	itemDir := filepath.Join(t.TempDir(), "mcp", "gh")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{"mcpServers":{"gh":{"command":"gh-mcp","alwaysAllow":["list"]}}}`), 0644)
	item := catalog.ContentItem{Name: "gh", Type: catalog.MCP, Provider: "cline", Path: itemDir}

	placement, err := installMCP(item, provider.Cursor, t.TempDir())
	if err != nil {
		t.Fatalf("installMCP: %v", err)
	}
	want := `gh: server "gh": autoApprove dropped (not documented by Cursor)`
	if got := conversionWarnings(placement.Notices); !slices.Contains(got, want) {
		t.Errorf("expected warning %q, got %q", want, got)
	}
}
