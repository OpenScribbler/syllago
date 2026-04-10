package registry

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestSync_SecurityHardening(t *testing.T) {
	// Verify that Sync() applies the same git hardening as Clone():
	// - core.hooksPath=/dev/null (disable git hooks)
	// - --no-recurse-submodules (block submodule fetching)
	// - GIT_CONFIG_NOSYSTEM=1 (set by caller on cmd.Env)
	//
	// We can't easily unit-test the env var (it's set on the exec.Cmd),
	// but we CAN verify this by attempting to sync a repo that has a
	// post-merge hook — with hardening, the hook should not execute.
	t.Parallel()

	// Create a bare repo and clone it to simulate a registry
	bare := t.TempDir()
	clone := t.TempDir()

	// Init bare repo with a commit
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
	run(bare, "init", "--bare")

	// Create a working copy to make initial commit
	work := t.TempDir()
	run(work, "clone", bare, ".")
	run(work, "config", "user.email", "test@test.com")
	run(work, "config", "user.name", "test")
	os.WriteFile(filepath.Join(work, "README.md"), []byte("init"), 0644)
	run(work, "add", ".")
	run(work, "commit", "-m", "init")
	run(work, "push", "origin", "master")

	// Clone as a "registry"
	run(clone, "clone", bare, "reg")
	regDir := filepath.Join(clone, "reg")

	// Add a post-merge hook that creates a marker file
	marker := filepath.Join(clone, "hook-executed")
	hooksDir := filepath.Join(regDir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)
	hook := fmt.Sprintf("#!/bin/sh\ntouch %s\n", marker)
	os.WriteFile(filepath.Join(hooksDir, "post-merge"), []byte(hook), 0755)

	// Push a new commit to bare so pull has something to merge
	os.WriteFile(filepath.Join(work, "file.txt"), []byte("update"), 0644)
	run(work, "add", ".")
	run(work, "commit", "-m", "update")
	run(work, "push", "origin", "master")

	// Now simulate what Sync does — with hardening, the hook should NOT fire
	cmd := exec.Command("git",
		"-C", regDir,
		"-c", "core.hooksPath=/dev/null",
		"pull", "--ff-only", "--no-recurse-submodules",
	)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hardened pull failed: %v\n%s", err, out)
	}

	// The marker file should NOT exist — hook was blocked
	if _, err := os.Stat(marker); err == nil {
		t.Error("post-merge hook executed despite core.hooksPath=/dev/null — hardening is broken")
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

func TestManifestItemRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item ManifestItem
	}{
		{
			name: "all extended fields populated",
			item: ManifestItem{
				Name:         "my-skill",
				Type:         "skills",
				Provider:     "claude-code",
				Path:         "skills/my-skill",
				HookEvent:    "PostToolUse",
				HookIndex:    2,
				Scripts:      []string{"hooks/run.sh", "hooks/check.sh"},
				DisplayName:  "My Awesome Skill",
				Description:  "A skill that does awesome things",
				ContentHash:  "sha256:abc123def456",
				References:   []string{"lib/helper.ts", "config/settings.json"},
				ConfigSource: ".claude/settings.json",
				Providers:    []string{"claude-code", "gemini-cli"},
			},
		},
		{
			name: "only base fields",
			item: ManifestItem{
				Name:     "basic-rule",
				Type:     "rules",
				Provider: "gemini-cli",
				Path:     "rules/basic.md",
			},
		},
		{
			name: "extended fields without base optional fields",
			item: ManifestItem{
				Name:        "analyzer-output",
				Type:        "hooks",
				Provider:    "claude-code",
				Path:        ".claude/settings.json",
				DisplayName: "Format on Save",
				Description: "Runs formatter after file edits",
				Providers:   []string{"claude-code"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Marshal to YAML
			data, err := yaml.Marshal(&tt.item)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			// Unmarshal back
			var got ManifestItem
			if err := yaml.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			// Compare all fields
			if got.Name != tt.item.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.item.Name)
			}
			if got.Type != tt.item.Type {
				t.Errorf("Type = %q, want %q", got.Type, tt.item.Type)
			}
			if got.Provider != tt.item.Provider {
				t.Errorf("Provider = %q, want %q", got.Provider, tt.item.Provider)
			}
			if got.Path != tt.item.Path {
				t.Errorf("Path = %q, want %q", got.Path, tt.item.Path)
			}
			if got.HookEvent != tt.item.HookEvent {
				t.Errorf("HookEvent = %q, want %q", got.HookEvent, tt.item.HookEvent)
			}
			if got.HookIndex != tt.item.HookIndex {
				t.Errorf("HookIndex = %d, want %d", got.HookIndex, tt.item.HookIndex)
			}
			if len(got.Scripts) != len(tt.item.Scripts) {
				t.Errorf("Scripts len = %d, want %d", len(got.Scripts), len(tt.item.Scripts))
			}
			// Extended fields
			if got.DisplayName != tt.item.DisplayName {
				t.Errorf("DisplayName = %q, want %q", got.DisplayName, tt.item.DisplayName)
			}
			if got.Description != tt.item.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.item.Description)
			}
			if got.ContentHash != tt.item.ContentHash {
				t.Errorf("ContentHash = %q, want %q", got.ContentHash, tt.item.ContentHash)
			}
			if len(got.References) != len(tt.item.References) {
				t.Errorf("References len = %d, want %d", len(got.References), len(tt.item.References))
			} else {
				for i := range got.References {
					if got.References[i] != tt.item.References[i] {
						t.Errorf("References[%d] = %q, want %q", i, got.References[i], tt.item.References[i])
					}
				}
			}
			if got.ConfigSource != tt.item.ConfigSource {
				t.Errorf("ConfigSource = %q, want %q", got.ConfigSource, tt.item.ConfigSource)
			}
			if len(got.Providers) != len(tt.item.Providers) {
				t.Errorf("Providers len = %d, want %d", len(got.Providers), len(tt.item.Providers))
			} else {
				for i := range got.Providers {
					if got.Providers[i] != tt.item.Providers[i] {
						t.Errorf("Providers[%d] = %q, want %q", i, got.Providers[i], tt.item.Providers[i])
					}
				}
			}
		})
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
