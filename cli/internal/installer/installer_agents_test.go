package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

const canonicalAgent = `---
name: reviewer
description: Reviews code
tools:
  - file_read
  - shell
---

Review the change for bugs.
`

// writeLibraryAgent creates a library agent the way syllago add stores it:
// <library>/agents/<name>/agent.md with no provider directory, so the catalog
// item carries an empty Provider.
func writeLibraryAgent(t *testing.T, root, content string) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(root, "library", "agents", "reviewer")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return catalog.ContentItem{Name: "reviewer", Type: catalog.Agents, Path: dir}
}

// TestInstall_AgentRendersProviderFormat is the regression test for agent
// installs linking the library file (at a path that did not exist) instead
// of writing the provider's format.
func TestInstall_AgentRendersProviderFormat(t *testing.T) {
	tests := []struct {
		prov    provider.Provider
		rel     string   // installed path relative to HOME
		want    []string // provider-format markers
		notWant []string
	}{
		{provider.ClaudeCode, ".claude/agents/reviewer.md", []string{"tools:", "- Read", "- Bash"}, []string{"file_read"}},
		{provider.Devin, ".config/devin/agents/reviewer.md", []string{"allowed-tools:", "- read", "- exec"}, []string{"file_read", "\ntools:"}},
		{provider.Cursor, ".cursor/agents/reviewer.md", []string{"name: reviewer", "Review the change"}, []string{"file_read"}},
		{provider.GeminiCLI, ".gemini/agents/reviewer.md", []string{"tools:", "- read_file", "- run_shell_command"}, []string{"file_read"}},
		{provider.Codex, ".codex/agents/reviewer.toml", []string{"[agent]", "developer_instructions"}, []string{"file_read", "---"}},
		{provider.CopilotCLI, ".github/agents/reviewer.agent.md", []string{"name: reviewer"}, []string{"file_read"}},
	}
	for _, tt := range tests {
		t.Run(tt.prov.Slug, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("HOME", tmp)
			item := writeLibraryAgent(t, tmp, canonicalAgent)

			for _, method := range []InstallMethod{MethodSymlink, MethodCopy} {
				placement, err := Install(item, tt.prov, tmp, method, "")
				if err != nil {
					t.Fatalf("Install(%s): %v", method, err)
				}
				wantPath := filepath.Join(tmp, tt.rel)
				if placement.Path != wantPath || placement.Mechanism != MechanismCopy {
					t.Fatalf("Install(%s) placement = %s %s, want copy %s", method, placement.Mechanism, placement.Path, wantPath)
				}
				info, err := os.Lstat(wantPath)
				if err != nil {
					t.Fatalf("installed file missing: %v", err)
				}
				if !info.Mode().IsRegular() {
					t.Fatalf("installed agent is %v, want a regular file", info.Mode())
				}
				data, _ := os.ReadFile(wantPath)
				for _, w := range tt.want {
					if !strings.Contains(string(data), w) {
						t.Errorf("installed agent missing %q:\n%s", w, data)
					}
				}
				for _, nw := range tt.notWant {
					if strings.Contains(string(data), nw) {
						t.Errorf("installed agent contains %q:\n%s", nw, data)
					}
				}
			}

			if got := CheckStatus(item, tt.prov, tmp); got != StatusInstalled {
				t.Errorf("CheckStatus after install = %v, want installed", got)
			}
			if _, err := Uninstall(item, tt.prov, tmp); err != nil {
				t.Fatalf("Uninstall: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(tmp, tt.rel)); !os.IsNotExist(err) {
				t.Errorf("agent still present after uninstall: %v", err)
			}
		})
	}
}

// TestInstall_AgentAddedWithoutType covers library agents stored in the
// source provider's format (add --all skips canonicalization): install must
// canonicalize from source_provider before rendering.
func TestInstall_AgentAddedWithoutType(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	item := writeLibraryAgent(t, tmp, "---\nname: reviewer\ndescription: Reviews code\ntools:\n  - Read\n  - Bash\n---\n\nReview.\n")
	item.Meta = &metadata.Meta{SourceProvider: "claude-code"}

	placement, err := Install(item, provider.Devin, tmp, MethodSymlink, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	data, _ := os.ReadFile(placement.Path)
	for _, w := range []string{"- read", "- exec"} {
		if !strings.Contains(string(data), w) {
			t.Errorf("devin agent missing %q:\n%s", w, data)
		}
	}
}

func TestInstall_AgentSameProviderCopiesSource(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	item := writeLibraryAgent(t, tmp, canonicalAgent)
	item.Meta = &metadata.Meta{SourceProvider: "codex"}
	original := "[agent]\nname = 'reviewer'\ndeveloper_instructions = 'original bytes'\n"
	srcDir := filepath.Join(item.Path, ".source")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "reviewer.toml"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	placement, err := Install(item, provider.Codex, tmp, MethodSymlink, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if want := filepath.Join(tmp, ".codex", "agents", "reviewer.toml"); placement.Path != want {
		t.Fatalf("path = %s, want %s", placement.Path, want)
	}
	if data, _ := os.ReadFile(placement.Path); string(data) != original {
		t.Errorf("same-provider install = %q, want the .source original", data)
	}
}

func TestInstall_AgentReplacesLegacySymlink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	item := writeLibraryAgent(t, tmp, canonicalAgent)
	target := filepath.Join(tmp, ".claude", "agents", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	// Earlier releases linked <item>/AGENT.md, which add never writes.
	if err := os.Symlink(filepath.Join(item.Path, "AGENT.md"), target); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(item, provider.ClaudeCode, tmp, MethodSymlink, ""); err != nil {
		t.Fatalf("Install: %v", err)
	}
	info, err := os.Lstat(target)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("target = %v, %v; want a regular file", info, err)
	}
	if _, err := os.Stat(filepath.Join(item.Path, "AGENT.md")); !os.IsNotExist(err) {
		t.Errorf("install wrote through the legacy symlink into the library")
	}
}

func TestInstall_AgentRefusesForeignSymlink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	item := writeLibraryAgent(t, tmp, canonicalAgent)
	target := filepath.Join(tmp, ".claude", "agents", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(tmp, "elsewhere.md")
	if err := os.Symlink(foreign, target); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(item, provider.ClaudeCode, tmp, MethodSymlink, ""); err == nil {
		t.Fatal("Install through a foreign symlink succeeded, want error")
	}
	if _, err := os.Stat(foreign); !os.IsNotExist(err) {
		t.Errorf("install wrote through the foreign symlink")
	}
}

func TestInstallWithResolver_AgentRendersProviderFormat(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	item := writeLibraryAgent(t, tmp, canonicalAgent)

	placement, err := InstallWithResolver(item, provider.Devin, tmp, MethodSymlink, config.NewResolver(nil, ""))
	if err != nil {
		t.Fatalf("InstallWithResolver: %v", err)
	}
	if placement.Mechanism != MechanismCopy || filepath.Base(placement.Path) != "reviewer.md" {
		t.Fatalf("placement = %s %s, want copy of reviewer.md", placement.Mechanism, placement.Path)
	}
	data, _ := os.ReadFile(placement.Path)
	if !strings.Contains(string(data), "allowed-tools:") {
		t.Errorf("devin agent not in devin format:\n%s", data)
	}
}

// A Kiro CLI JSON original must not be copied into Kiro's .md agent file.
func TestInstall_AgentSameProviderRendersWhenSourceFormatDiffers(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	item := writeLibraryAgent(t, tmp, canonicalAgent)
	item.Meta = &metadata.Meta{SourceProvider: "kiro"}
	srcDir := filepath.Join(item.Path, ".source")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "reviewer.json"), []byte(`{"name":"reviewer","prompt":"json original"}`), 0644); err != nil {
		t.Fatal(err)
	}

	placement, err := Install(item, provider.Kiro, tmp, MethodCopy, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !strings.HasSuffix(placement.Path, "reviewer.md") {
		t.Fatalf("path = %s, want reviewer.md", placement.Path)
	}
	data, err := os.ReadFile(placement.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		t.Errorf("kiro .md agent holds the JSON original:\n%s", data)
	}
}

// A rendered install replaces the destination rather than writing through it,
// so a hard link back to the library file cannot rewrite the library.
func TestInstall_AgentDoesNotWriteThroughHardLink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	item := writeLibraryAgent(t, tmp, canonicalAgent)
	libFile := filepath.Join(item.Path, "agent.md")
	target := filepath.Join(tmp, ".gemini", "agents", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(libFile, target); err != nil {
		t.Skipf("hard links unsupported: %v", err)
	}

	if _, err := Install(item, provider.GeminiCLI, tmp, MethodCopy, ""); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if data, _ := os.ReadFile(libFile); string(data) != canonicalAgent {
		t.Errorf("library file changed through hard link:\n%s", data)
	}
}
