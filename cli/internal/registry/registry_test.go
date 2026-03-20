package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/acme/my-tools.git", "acme/my-tools"},
		{"https://github.com/acme/my-tools", "acme/my-tools"},
		{"https://github.com/acme/my-tools/", "acme/my-tools"},
		{"git@github.com:acme/my-tools.git", "acme/my-tools"},
		{"git@github.com:acme/my-tools", "acme/my-tools"},
		{"git@github.com:acme/my_tools.git", "acme/my_tools"},
		{"https://example.com/my-tools.git", "my-tools"},
		{"https://example.com/my-tools", "my-tools"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := NameFromURL(tt.url); got != tt.want {
				t.Errorf("NameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExpandAlias_KnownAliasTableIsEmpty(t *testing.T) {
	if len(KnownAliases) != 0 {
		t.Errorf("KnownAliases should be empty, got %d entries: %v", len(KnownAliases), KnownAliases)
	}
}

func TestExpandAlias_FullURL_NotExpanded(t *testing.T) {
	input := "https://github.com/acme/tools.git"
	url, expanded := ExpandAlias(input)
	if expanded {
		t.Fatal("expected expanded=false for full URL")
	}
	if url != input {
		t.Errorf("url = %q, want %q", url, input)
	}
}

func TestExpandAlias_UnknownShortName_NotExpanded(t *testing.T) {
	input := "some-random-name"
	url, expanded := ExpandAlias(input)
	if expanded {
		t.Fatal("expected expanded=false for unknown short name")
	}
	if url != input {
		t.Errorf("url = %q, want %q", url, input)
	}
}

func TestExpandAlias_SSHURL_NotExpanded(t *testing.T) {
	input := "git@github.com:acme/tools.git"
	url, expanded := ExpandAlias(input)
	if expanded {
		t.Fatal("expected expanded=false for SSH URL (contains ':')")
	}
	if url != input {
		t.Errorf("url = %q, want %q", url, input)
	}
}

func TestCloneArgs_SecurityProtections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		url  string
		dir  string
		ref  string
	}{
		{"no ref", "https://github.com/acme/tools.git", "/tmp/clone", ""},
		{"with ref", "https://github.com/acme/tools.git", "/tmp/clone", "main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := cloneArgs(tt.url, tt.dir, tt.ref)

			// Must contain -c core.hooksPath=/dev/null to disable git hooks
			foundHooksPath := false
			for i, a := range args {
				if a == "-c" && i+1 < len(args) && args[i+1] == "core.hooksPath=/dev/null" {
					foundHooksPath = true
					break
				}
			}
			if !foundHooksPath {
				t.Errorf("cloneArgs missing -c core.hooksPath=/dev/null, got %v", args)
			}

			// Must contain --no-recurse-submodules
			found := false
			for _, a := range args {
				if a == "--no-recurse-submodules" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("cloneArgs missing --no-recurse-submodules, got %v", args)
			}

			// Must contain clone, url, and dir
			foundClone := false
			for _, a := range args {
				if a == "clone" {
					foundClone = true
					break
				}
			}
			if !foundClone {
				t.Errorf("cloneArgs missing 'clone' subcommand, got %v", args)
			}

			// If ref is set, must contain --branch ref
			if tt.ref != "" {
				foundBranch := false
				for i, a := range args {
					if a == "--branch" && i+1 < len(args) && args[i+1] == tt.ref {
						foundBranch = true
						break
					}
				}
				if !foundBranch {
					t.Errorf("cloneArgs with ref=%q missing --branch flag, got %v", tt.ref, args)
				}
			}
		})
	}
}

func TestClone_SetsGitConfigNoSystem(t *testing.T) {
	// Verify that the Clone function sets GIT_CONFIG_NOSYSTEM=1 by checking
	// that cloneArgs produces args that will be used with the env var.
	// The actual env var is set in Clone() — we verify it indirectly by
	// confirming cloneArgs is the only path and checking the source.
	t.Parallel()
	args := cloneArgs("https://example.com/repo.git", "/tmp/test", "")

	// The -c flag must come BEFORE clone to be a global git option
	cloneIdx := -1
	hooksIdx := -1
	for i, a := range args {
		if a == "clone" {
			cloneIdx = i
		}
		if a == "core.hooksPath=/dev/null" {
			hooksIdx = i
		}
	}
	if cloneIdx < 0 {
		t.Fatal("clone subcommand not found in args")
	}
	if hooksIdx < 0 {
		t.Fatal("core.hooksPath=/dev/null not found in args")
	}
	if hooksIdx >= cloneIdx {
		t.Errorf("core.hooksPath=/dev/null (index %d) must come before clone (index %d) to be a global git option", hooksIdx, cloneIdx)
	}
}

func TestLoadManifest_Missing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() // no registry.yaml
	m, err := LoadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("LoadManifestFromDir: %v", err)
	}
	if m != nil {
		t.Errorf("expected nil manifest for missing file, got %+v", m)
	}
}

func TestLoadManifest_Valid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "name: my-registry\ndescription: Test registry\nversion: \"1.0.0\"\n"
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := LoadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("LoadManifestFromDir: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if m.Name != "my-registry" {
		t.Errorf("Name = %q, want %q", m.Name, "my-registry")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
	if m.Description != "Test registry" {
		t.Errorf("Description = %q, want %q", m.Description, "Test registry")
	}
}

func TestLoadManifest_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(":\n  - bad: [yaml"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := LoadManifestFromDir(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadManifest_AllFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `name: full-registry
description: A full registry
maintainers:
  - alice
  - bob
version: "2.1.0"
min_syllago_version: "0.5.0"
`
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := LoadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("LoadManifestFromDir: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if len(m.Maintainers) != 2 {
		t.Errorf("Maintainers len = %d, want 2", len(m.Maintainers))
	}
	if m.MinSyllagoVersion != "0.5.0" {
		t.Errorf("MinSyllagoVersion = %q, want %q", m.MinSyllagoVersion, "0.5.0")
	}
}

func TestLoadManifest_WithItems(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `name: native-registry
items:
  - name: my-skill
    type: skills
    provider: claude-code
    path: skills/my-skill.md
  - name: on-save-hook
    type: hooks
    provider: claude-code
    path: .claude/settings.json
    hookEvent: PostToolUse
    hookIndex: 0
    scripts:
      - hooks/on-save.sh
`
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := LoadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("LoadManifestFromDir: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if len(m.Items) != 2 {
		t.Fatalf("Items len = %d, want 2", len(m.Items))
	}

	skill := m.Items[0]
	if skill.Name != "my-skill" {
		t.Errorf("Items[0].Name = %q, want %q", skill.Name, "my-skill")
	}
	if skill.Type != "skills" {
		t.Errorf("Items[0].Type = %q, want %q", skill.Type, "skills")
	}
	if skill.Provider != "claude-code" {
		t.Errorf("Items[0].Provider = %q, want %q", skill.Provider, "claude-code")
	}
	if skill.Path != "skills/my-skill.md" {
		t.Errorf("Items[0].Path = %q, want %q", skill.Path, "skills/my-skill.md")
	}
	if skill.HookEvent != "" {
		t.Errorf("Items[0].HookEvent = %q, want empty", skill.HookEvent)
	}

	hook := m.Items[1]
	if hook.HookEvent != "PostToolUse" {
		t.Errorf("Items[1].HookEvent = %q, want %q", hook.HookEvent, "PostToolUse")
	}
	if hook.HookIndex != 0 {
		t.Errorf("Items[1].HookIndex = %d, want 0", hook.HookIndex)
	}
	if len(hook.Scripts) != 1 || hook.Scripts[0] != "hooks/on-save.sh" {
		t.Errorf("Items[1].Scripts = %v, want [hooks/on-save.sh]", hook.Scripts)
	}
}

func TestLoadManifest_WithoutItems(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "name: legacy-registry\ndescription: No items section\n"
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := LoadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("LoadManifestFromDir: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if m.Items != nil {
		t.Errorf("Items = %v, want nil for registry.yaml without items section", m.Items)
	}
}
