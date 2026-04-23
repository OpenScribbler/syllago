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

// setupGlobalSkillForEdit creates a global library with a single skill item.
// Skills are universal type (no provider subdirectory needed).
// Returns the global dir and the skill's directory path.
func setupGlobalSkillForEdit(t *testing.T, name string) (globalDir, skillDir string) {
	t.Helper()
	globalDir = t.TempDir()
	skillDir = filepath.Join(globalDir, "skills", name)
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name+"\nA test skill.\n"), 0644)
	return globalDir, skillDir
}

func TestRunEdit_NameOnly(t *testing.T) {
	// Not parallel — mutates globals (GlobalContentDirOverride, output.*)
	globalDir, skillDir := setupGlobalSkillForEdit(t, "my-skill-edit")
	withGlobalLibrary(t, globalDir)
	stdout, _ := output.SetForTest(t)

	editCmd.Flags().Set("name", "Better Skill Name")
	defer editCmd.Flags().Set("name", "")

	err := editCmd.RunE(editCmd, []string{"my-skill-edit"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Better Skill Name") {
		t.Errorf("expected new name in output, got: %s", out)
	}

	meta, err := metadata.Load(skillDir)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata to exist after edit")
	}
	if meta.Name != "Better Skill Name" {
		t.Errorf("expected metadata name %q, got %q", "Better Skill Name", meta.Name)
	}
}

func TestRunEdit_DescriptionOnly(t *testing.T) {
	globalDir, skillDir := setupGlobalSkillForEdit(t, "desc-skill")
	withGlobalLibrary(t, globalDir)
	stdout, _ := output.SetForTest(t)

	editCmd.Flags().Set("description", "A great skill for testing")
	defer editCmd.Flags().Set("description", "")

	err := editCmd.RunE(editCmd, []string{"desc-skill"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "description set") {
		t.Errorf("expected 'description set' in output, got: %s", out)
	}

	meta, err := metadata.Load(skillDir)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata to exist after edit")
	}
	if meta.Description != "A great skill for testing" {
		t.Errorf("expected description %q, got %q", "A great skill for testing", meta.Description)
	}
}

func TestRunEdit_NoFlagsErrors(t *testing.T) {
	globalDir, _ := setupGlobalSkillForEdit(t, "no-flags-skill")
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	err := editCmd.RunE(editCmd, []string{"no-flags-skill"})
	if err == nil {
		t.Fatal("expected error when neither --name nor --description provided")
	}
	if !strings.Contains(err.Error(), "INPUT_001") {
		t.Errorf("expected INPUT_001 error code, got: %v", err)
	}
}

func TestRunEdit_NonExistentItem(t *testing.T) {
	globalDir := t.TempDir()
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	editCmd.Flags().Set("name", "Whatever")
	defer editCmd.Flags().Set("name", "")

	err := editCmd.RunE(editCmd, []string{"does-not-exist"})
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

func TestRunEdit_NoArgs(t *testing.T) {
	_, _ = output.SetForTest(t)

	// cobra.ExactArgs(1) rejects empty args before RunE runs.
	err := editCmd.Args(editCmd, []string{})
	if err == nil {
		t.Fatal("expected error for no args")
	}
}

func TestRunEdit_JSONOutput(t *testing.T) {
	globalDir, _ := setupGlobalSkillForEdit(t, "json-skill-edit")
	withGlobalLibrary(t, globalDir)
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	editCmd.Flags().Set("name", "JSON Skill Name")
	defer editCmd.Flags().Set("name", "")

	err := editCmd.RunE(editCmd, []string{"json-skill-edit"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result editResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, stdout.String())
	}
	if result.Item != "json-skill-edit" {
		t.Errorf("expected item %q, got %q", "json-skill-edit", result.Item)
	}
	if result.NewName != "JSON Skill Name" {
		t.Errorf("expected new_name %q, got %q", "JSON Skill Name", result.NewName)
	}
	if result.Type != "skills" {
		t.Errorf("expected type %q, got %q", "skills", result.Type)
	}
}

func TestRunEdit_JSONOutputDescription(t *testing.T) {
	globalDir, _ := setupGlobalSkillForEdit(t, "json-desc-skill")
	withGlobalLibrary(t, globalDir)
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	editCmd.Flags().Set("description", "A JSON description")
	defer editCmd.Flags().Set("description", "")

	err := editCmd.RunE(editCmd, []string{"json-desc-skill"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result editResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, stdout.String())
	}
	if result.Description != "A JSON description" {
		t.Errorf("expected description %q, got %q", "A JSON description", result.Description)
	}
	if result.NewName != "" {
		t.Errorf("expected no new_name when only description set, got %q", result.NewName)
	}
}

func TestRunEdit_AmbiguousItem(t *testing.T) {
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

	editCmd.Flags().Set("name", "Disambiguated")
	defer editCmd.Flags().Set("name", "")

	err := editCmd.RunE(editCmd, []string{"shared-name"})
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

func TestRunEdit_TypeFilter(t *testing.T) {
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

	editCmd.Flags().Set("name", "Skills Only")
	editCmd.Flags().Set("type", "skills")
	defer editCmd.Flags().Set("name", "")
	defer editCmd.Flags().Set("type", "")

	err := editCmd.RunE(editCmd, []string{"multi-name"})
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
