package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// setupGlobalSkillForRename creates a global library with a single skill item.
// Skills are universal type (no provider subdirectory needed).
// Returns the global dir and the skill's directory path.
func setupGlobalSkillForRename(t *testing.T, name string) (globalDir, skillDir string) {
	t.Helper()
	globalDir = t.TempDir()
	skillDir = filepath.Join(globalDir, "skills", name)
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name+"\nA test skill.\n"), 0644)
	return globalDir, skillDir
}

func TestRunRename_Success(t *testing.T) {
	// Not parallel — mutates globals (GlobalContentDirOverride, output.*)
	globalDir, skillDir := setupGlobalSkillForRename(t, "my-skill-rename")
	withGlobalLibrary(t, globalDir)
	stdout, _ := output.SetForTest(t)

	renameCmd.Flags().Set("name", "Better Skill Name")
	defer renameCmd.Flags().Set("name", "")

	err := renameCmd.RunE(renameCmd, []string{"my-skill-rename"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify output mentions the rename
	out := stdout.String()
	if !strings.Contains(out, "Better Skill Name") {
		t.Errorf("expected new name in output, got: %s", out)
	}

	// Verify metadata was actually saved to disk
	meta, err := metadata.Load(skillDir)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata to exist after rename")
	}
	if meta.Name != "Better Skill Name" {
		t.Errorf("expected metadata name %q, got %q", "Better Skill Name", meta.Name)
	}
}

func TestRunRename_NonExistentItem(t *testing.T) {
	globalDir := t.TempDir()
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	renameCmd.Flags().Set("name", "Whatever")
	defer renameCmd.Flags().Set("name", "")

	err := renameCmd.RunE(renameCmd, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("expected error for non-existent item")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("expected item name in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ITEM_001") {
		t.Errorf("expected ITEM_001 error code, got: %v", err)
	}
}

func TestRunRename_NoArgs(t *testing.T) {
	_, _ = output.SetForTest(t)

	// cobra.ExactArgs(1) rejects empty args before RunE runs.
	err := renameCmd.Args(renameCmd, []string{})
	if err == nil {
		t.Fatal("expected error for no args")
	}
}

func TestRunRename_JSONOutput(t *testing.T) {
	globalDir, _ := setupGlobalSkillForRename(t, "json-skill")
	withGlobalLibrary(t, globalDir)
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	renameCmd.Flags().Set("name", "JSON Skill Name")
	defer renameCmd.Flags().Set("name", "")

	err := renameCmd.RunE(renameCmd, []string{"json-skill"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result renameResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, stdout.String())
	}
	if result.Item != "json-skill" {
		t.Errorf("expected item %q, got %q", "json-skill", result.Item)
	}
	if result.NewName != "JSON Skill Name" {
		t.Errorf("expected new_name %q, got %q", "JSON Skill Name", result.NewName)
	}
	if result.Type != "skills" {
		t.Errorf("expected type %q, got %q", "skills", result.Type)
	}
}

func TestRunRename_AmbiguousItem(t *testing.T) {
	// Create an item that exists in two universal types (skills and agents)
	globalDir := t.TempDir()
	skillDir := filepath.Join(globalDir, "skills", "shared-name")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill\n"), 0644)
	agentDir := filepath.Join(globalDir, "agents", "shared-name")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte("# agent\n"), 0644)

	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	renameCmd.Flags().Set("name", "Disambiguated")
	defer renameCmd.Flags().Set("name", "")

	err := renameCmd.RunE(renameCmd, []string{"shared-name"})
	if err == nil {
		t.Fatal("expected error for ambiguous item")
	}
	if !strings.Contains(err.Error(), "ITEM_002") {
		t.Errorf("expected ITEM_002 error code, got: %v", err)
	}
	if !strings.Contains(err.Error(), "multiple types") {
		t.Errorf("expected 'multiple types' in error, got: %v", err)
	}
}

func TestRunRename_TypeFilter(t *testing.T) {
	// Create an item in two types, then use --type to disambiguate
	globalDir := t.TempDir()
	skillDir := filepath.Join(globalDir, "skills", "multi-name")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill\n"), 0644)
	agentDir := filepath.Join(globalDir, "agents", "multi-name")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte("# agent\n"), 0644)

	withGlobalLibrary(t, globalDir)
	stdout, _ := output.SetForTest(t)

	renameCmd.Flags().Set("name", "Skills Only")
	renameCmd.Flags().Set("type", "skills")
	defer renameCmd.Flags().Set("name", "")
	defer renameCmd.Flags().Set("type", "")

	err := renameCmd.RunE(renameCmd, []string{"multi-name"})
	if err != nil {
		t.Fatalf("expected no error with --type filter, got: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Skills Only") {
		t.Errorf("expected new name in output, got: %s", out)
	}

	// Verify only the skills metadata was updated
	meta, err := metadata.Load(skillDir)
	if err != nil {
		t.Fatalf("loading skill metadata: %v", err)
	}
	if meta == nil || meta.Name != "Skills Only" {
		t.Errorf("expected skill metadata name %q, got %v", "Skills Only", meta)
	}

	// Agent metadata should not have been updated
	agentMeta, err := metadata.Load(agentDir)
	if err != nil {
		t.Fatalf("loading agent metadata: %v", err)
	}
	if agentMeta != nil && agentMeta.Name == "Skills Only" {
		t.Error("agent metadata should not have been renamed")
	}
}
