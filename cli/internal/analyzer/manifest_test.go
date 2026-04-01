package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"gopkg.in/yaml.v3"
)

func TestToManifestItem(t *testing.T) {
	t.Parallel()

	item := &DetectedItem{
		Name:         "my-hook",
		Type:         catalog.Hooks,
		Provider:     "claude-code",
		Path:         "hooks/my-hook.sh",
		HookEvent:    "before_tool_execute",
		HookIndex:    1,
		Scripts:      []string{"hooks/my-hook.sh"},
		DisplayName:  "My Hook",
		Description:  "Runs before tool execution",
		ContentHash:  "abc123",
		References:   []string{"lib/helper.ts"},
		ConfigSource: ".claude/settings.json",
		Providers:    []string{"hooks/alt-path.sh"},
	}

	mi := ToManifestItem(item)

	if mi.Name != "my-hook" {
		t.Errorf("Name = %q, want %q", mi.Name, "my-hook")
	}
	if mi.Type != "hooks" {
		t.Errorf("Type = %q, want %q", mi.Type, "hooks")
	}
	if mi.Provider != "claude-code" {
		t.Errorf("Provider = %q, want %q", mi.Provider, "claude-code")
	}
	if mi.HookEvent != "before_tool_execute" {
		t.Errorf("HookEvent = %q, want %q", mi.HookEvent, "before_tool_execute")
	}
	if mi.HookIndex != 1 {
		t.Errorf("HookIndex = %d, want 1", mi.HookIndex)
	}
	if len(mi.Scripts) != 1 || mi.Scripts[0] != "hooks/my-hook.sh" {
		t.Errorf("Scripts = %v, want [hooks/my-hook.sh]", mi.Scripts)
	}
	if mi.DisplayName != "My Hook" {
		t.Errorf("DisplayName = %q, want %q", mi.DisplayName, "My Hook")
	}
	if mi.Description != "Runs before tool execution" {
		t.Errorf("Description = %q, want %q", mi.Description, "Runs before tool execution")
	}
	if mi.ContentHash != "abc123" {
		t.Errorf("ContentHash = %q, want %q", mi.ContentHash, "abc123")
	}
	if mi.ConfigSource != ".claude/settings.json" {
		t.Errorf("ConfigSource = %q, want %q", mi.ConfigSource, ".claude/settings.json")
	}
	if len(mi.Providers) != 1 || mi.Providers[0] != "hooks/alt-path.sh" {
		t.Errorf("Providers = %v, want [hooks/alt-path.sh]", mi.Providers)
	}
}

func TestWriteGeneratedManifest(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	orig := registry.CacheDirOverride
	registry.CacheDirOverride = tmp
	t.Cleanup(func() { registry.CacheDirOverride = orig })

	items := []*DetectedItem{
		{Name: "my-skill", Type: catalog.Skills, Provider: "syllago", Path: "skills/my-skill", DisplayName: "My Skill"},
		{Name: "my-hook", Type: catalog.Hooks, Provider: "claude-code", Path: ".claude/settings.json", HookEvent: "before_tool_execute"},
	}

	err := WriteGeneratedManifest("acme/tools", items)
	if err != nil {
		t.Fatalf("WriteGeneratedManifest error: %v", err)
	}

	// Verify the file exists.
	dest := filepath.Join(tmp, "acme/tools", "registry.yaml")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	// Round-trip parse.
	var m registry.Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if m.Version != "1" {
		t.Errorf("Version = %q, want %q", m.Version, "1")
	}
	if len(m.Items) != 2 {
		t.Fatalf("Items len = %d, want 2", len(m.Items))
	}
	if m.Items[0].Name != "my-skill" {
		t.Errorf("Items[0].Name = %q, want %q", m.Items[0].Name, "my-skill")
	}
	if m.Items[0].DisplayName != "My Skill" {
		t.Errorf("Items[0].DisplayName = %q, want %q", m.Items[0].DisplayName, "My Skill")
	}
}
