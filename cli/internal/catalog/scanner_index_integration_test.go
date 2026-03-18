package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNativeRegistryRoundTrip verifies the full pipeline:
// native content → registry.yaml with items → ScanRegistriesOnly → items found
func TestNativeRegistryRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Create Claude Code native structure
	skillDir := filepath.Join(dir, ".claude", "skills", "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: Test Skill\ndescription: A test skill\n---\n# Instructions\nDo the thing."), 0644)
	os.WriteFile(filepath.Join(skillDir, "helper.md"), []byte("# Helper"), 0644)

	agentDir := filepath.Join(dir, ".claude", "agents")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "test-agent.md"), []byte("---\nname: Test Agent\ndescription: An agent\n---\n# Agent"), 0644)

	cmdDir := filepath.Join(dir, ".claude", "commands")
	os.MkdirAll(cmdDir, 0755)
	os.WriteFile(filepath.Join(cmdDir, "deploy.md"), []byte("Deploy the thing"), 0644)

	// Step 1: Scan native content
	result := ScanNativeContent(dir)
	if result.HasSyllagoStructure {
		t.Fatal("should not detect syllago structure in native repo")
	}
	if len(result.Providers) == 0 {
		t.Fatal("should find providers")
	}

	// Step 2: Build registry.yaml from scan results
	var yamlItems []string
	for _, prov := range result.Providers {
		for typeLabel, items := range prov.Items {
			for _, item := range items {
				yamlItems = append(yamlItems, "  - name: "+item.Name+"\n    type: "+typeLabel+"\n    provider: "+prov.ProviderSlug+"\n    path: "+item.Path)
			}
		}
	}

	manifest := "name: test-native-registry\nitems:\n"
	for _, yi := range yamlItems {
		manifest += yi + "\n"
	}
	os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(manifest), 0644)

	// Step 3: Scan as registry
	sources := []RegistrySource{{Name: "test-native", Path: dir}}
	cat, err := ScanRegistriesOnly(sources)
	if err != nil {
		t.Fatal(err)
	}

	if len(cat.Warnings) > 0 {
		for _, w := range cat.Warnings {
			t.Logf("warning: %s", w)
		}
	}

	// Step 4: Verify items
	if len(cat.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(cat.Items))
	}

	found := map[string]bool{}
	for _, item := range cat.Items {
		found[item.Name] = true
		if item.Registry != "test-native" {
			t.Errorf("item %q: registry=%q, want test-native", item.Name, item.Registry)
		}
	}

	if !found["test-skill"] {
		t.Error("missing test-skill")
	}
	if !found["test-agent"] {
		t.Error("missing test-agent")
	}
	if !found["deploy"] {
		t.Error("missing deploy")
	}

	// Verify skill metadata was extracted
	for _, item := range cat.Items {
		if item.Name == "test-skill" {
			if item.DisplayName != "Test Skill" {
				t.Errorf("skill DisplayName=%q, want 'Test Skill'", item.DisplayName)
			}
			if item.Description != "A test skill" {
				t.Errorf("skill Description=%q, want 'A test skill'", item.Description)
			}
			if len(item.Files) != 2 {
				t.Errorf("skill files: expected 2, got %d: %v", len(item.Files), item.Files)
			}
		}
	}
}

// TestNativeRegistryCoexistsWithNative verifies that syllago-native registries
// still work when the indexed registry code path exists.
func TestNativeRegistryCoexistsWithNative(t *testing.T) {
	// Registry A: syllago-native layout
	dirA := t.TempDir()
	skillDir := filepath.Join(dirA, "skills", "hello")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: Hello\n---\n"), 0644)

	// Registry B: indexed native layout
	dirB := t.TempDir()
	nativeSkill := filepath.Join(dirB, ".claude", "skills", "world")
	os.MkdirAll(nativeSkill, 0755)
	os.WriteFile(filepath.Join(nativeSkill, "SKILL.md"), []byte("---\nname: World\n---\n"), 0644)
	os.WriteFile(filepath.Join(dirB, "registry.yaml"), []byte("name: b\nitems:\n  - name: world\n    type: skills\n    provider: claude-code\n    path: .claude/skills/world\n"), 0644)

	sources := []RegistrySource{
		{Name: "reg-a", Path: dirA},
		{Name: "reg-b", Path: dirB},
	}
	cat, err := ScanRegistriesOnly(sources)
	if err != nil {
		t.Fatal(err)
	}

	if len(cat.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(cat.Items))
	}

	registries := map[string]string{}
	for _, item := range cat.Items {
		registries[item.Name] = item.Registry
	}

	if registries["hello"] != "reg-a" {
		t.Errorf("hello: registry=%q", registries["hello"])
	}
	if registries["world"] != "reg-b" {
		t.Errorf("world: registry=%q", registries["world"])
	}
}
