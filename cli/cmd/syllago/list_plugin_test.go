package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/ccplugin"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/tidwall/gjson"
)

func setupListPluginRepo(t *testing.T, librarySkillName string) (string, string) {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills"), 0755); err != nil {
		t.Fatalf("mkdir repo marker: %v", err)
	}

	globalDir := t.TempDir()
	skillDir := filepath.Join(globalDir, "skills", librarySkillName)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill fixture: %v", err)
	}
	skillFrontmatter := "---\nname: " + librarySkillName + "\ndescription: Format code\n---\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillFrontmatter), 0644); err != nil {
		t.Fatalf("write skill fixture: %v", err)
	}
	return root, globalDir
}

func setupListPluginTest(t *testing.T, plugins []ccplugin.Plugin) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	return setupListPluginTestWithLibrarySkill(t, plugins, "library-format")
}

func setupListPluginTestWithLibrarySkill(t *testing.T, plugins []ccplugin.Plugin, librarySkillName string) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	root, globalDir := setupListPluginRepo(t, librarySkillName)
	withFakeRepoRoot(t, root)
	withGlobalLibrary(t, globalDir)
	isolateListEnv(t)

	stdout, stderr := output.SetForTest(t)
	output.JSON = false

	origLoad := loadCCPlugins
	loadCCPlugins = func() ([]ccplugin.Plugin, error) {
		return plugins, nil
	}
	t.Cleanup(func() { loadCCPlugins = origLoad })

	listCmd.Flags().Set("source", "all")
	listCmd.Flags().Set("type", "")
	listCmd.Flags().Set("filter", "")
	t.Cleanup(func() {
		listCmd.Flags().Set("source", "all")
		listCmd.Flags().Set("type", "")
		listCmd.Flags().Set("filter", "")
		output.JSON = false
	})

	return stdout, stderr
}

func captureProcessStderr(t *testing.T) func() string {
	t.Helper()

	oldStderr := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stderr = writePipe

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, readPipe)
		close(done)
	}()

	var once sync.Once
	restore := func() string {
		once.Do(func() {
			os.Stderr = oldStderr
			_ = writePipe.Close()
			<-done
			_ = readPipe.Close()
		})
		return buf.String()
	}
	t.Cleanup(func() { _ = restore() })
	return restore
}

func testListPlugins() []ccplugin.Plugin {
	return []ccplugin.Plugin{
		{
			Name:        "fmt-tools",
			Marketplace: "acme-marketplace",
			ID:          "fmt-tools@acme-marketplace",
			Enabled:     true,
			Version:     "1.2.0",
			Skills: []ccplugin.Skill{
				{Name: "format", Path: "/plugins/fmt-tools/skills/format"},
				{Name: "lint", Path: "/plugins/fmt-tools/skills/lint"},
			},
		},
		{
			Name:        "notes",
			Marketplace: "acme-marketplace",
			ID:          "notes@acme-marketplace",
			Enabled:     true,
			Skills: []ccplugin.Skill{
				{Name: "notes", Path: "/plugins/notes/skills/notes"},
			},
		},
	}
}

func TestListPluginsSourceAllTextAndJSON(t *testing.T) {
	stdout, _ := setupListPluginTest(t, testListPlugins())

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Skills (1)") {
		t.Fatalf("expected catalog group in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Claude Code plugins (2)") {
		t.Fatalf("expected plugin section in output, got:\n%s", out)
	}
	if !strings.Contains(out, "fmt-tools@acme-marketplace") || !strings.Contains(out, "v1.2.0  skills: format, lint") {
		t.Fatalf("expected fmt-tools plugin details in output, got:\n%s", out)
	}
	if !strings.Contains(out, "notes@acme-marketplace") || !strings.Contains(out, "skills: notes") {
		t.Fatalf("expected notes plugin details in output, got:\n%s", out)
	}

	stdout, _ = setupListPluginTest(t, testListPlugins())
	output.JSON = true

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list --json failed: %v", err)
	}

	raw := stdout.Bytes()
	if got := gjson.GetBytes(raw, "plugins.#").Int(); got != 2 {
		t.Fatalf("plugins length = %d, want 2; raw: %s", got, stdout.String())
	}
	if got := gjson.GetBytes(raw, "plugins.0.name").String(); got != "fmt-tools" {
		t.Errorf("plugins.0.name = %q, want fmt-tools", got)
	}
	if got := gjson.GetBytes(raw, "plugins.0.marketplace").String(); got != "acme-marketplace" {
		t.Errorf("plugins.0.marketplace = %q, want acme-marketplace", got)
	}
	if got := gjson.GetBytes(raw, "plugins.0.version").String(); got != "1.2.0" {
		t.Errorf("plugins.0.version = %q, want 1.2.0", got)
	}
	if got := gjson.GetBytes(raw, "plugins.0.skills.0").String(); got != "format" {
		t.Errorf("plugins.0.skills.0 = %q, want format", got)
	}
	if got := gjson.GetBytes(raw, "plugins.0.skills.1").String(); got != "lint" {
		t.Errorf("plugins.0.skills.1 = %q, want lint", got)
	}
}

func TestListPluginsSourcePluginOnly(t *testing.T) {
	stdout, _ := setupListPluginTest(t, testListPlugins())
	listCmd.Flags().Set("source", "plugin")

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list --source plugin failed: %v", err)
	}

	out := stdout.String()
	if strings.Contains(out, "Skills (") {
		t.Fatalf("catalog groups should be absent for --source plugin, got:\n%s", out)
	}
	if !strings.Contains(out, "Claude Code plugins (2)") {
		t.Fatalf("expected plugin section, got:\n%s", out)
	}
}

func TestListPluginsSourceLibraryHidesPluginSection(t *testing.T) {
	stdout, _ := setupListPluginTest(t, testListPlugins())
	listCmd.Flags().Set("source", "library")

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list --source library failed: %v", err)
	}

	if strings.Contains(stdout.String(), "Claude Code plugins") {
		t.Fatalf("plugin section should be absent for --source library, got:\n%s", stdout.String())
	}
}

func TestListPluginsTypeFilterHidesPluginSection(t *testing.T) {
	stdout, _ := setupListPluginTest(t, testListPlugins())
	listCmd.Flags().Set("type", "skills")

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list --type skills failed: %v", err)
	}

	if strings.Contains(stdout.String(), "Claude Code plugins") {
		t.Fatalf("plugin section should be absent for --type skills, got:\n%s", stdout.String())
	}
}

func TestListPluginsDisabledNeverShown(t *testing.T) {
	stdout, _ := setupListPluginTest(t, []ccplugin.Plugin{
		{
			Name:        "disabled",
			Marketplace: "acme-marketplace",
			ID:          "disabled@acme-marketplace",
			Enabled:     false,
			Skills:      []ccplugin.Skill{{Name: "disabled", Path: "/plugins/disabled/skills/disabled"}},
		},
	})

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if strings.Contains(stdout.String(), "disabled@acme-marketplace") {
		t.Fatalf("disabled plugin should not be shown, got:\n%s", stdout.String())
	}
}

func TestListPluginsCollisionWarnsWithoutSwallowingLibrarySkill(t *testing.T) {
	stdout, _ := setupListPluginTestWithLibrarySkill(t, []ccplugin.Plugin{
		{
			Name:        "fmt-tools",
			Marketplace: "acme-marketplace",
			ID:          "fmt-tools@acme-marketplace",
			Enabled:     true,
			Skills:      []ccplugin.Skill{{Name: "Format", Path: "/plugins/fmt-tools/skills/format"}},
		},
	}, "format")
	restoreStderr := captureProcessStderr(t)

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	errOut := restoreStderr()
	if !strings.Contains(errOut, "shadows a library skill") {
		t.Fatalf("expected collision warning, got stderr:\n%s", errOut)
	}
	if !strings.Contains(stdout.String(), "format") {
		t.Fatalf("library skill row should still appear, got:\n%s", stdout.String())
	}
}

func TestListPluginsLoadErrorWarnsAndContinues(t *testing.T) {
	stdout, stderr := setupListPluginTest(t, nil)

	origLoad := loadCCPlugins
	loadCCPlugins = func() ([]ccplugin.Plugin, error) {
		return nil, errors.New("fixture boom")
	}
	t.Cleanup(func() { loadCCPlugins = origLoad })

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if !strings.Contains(stderr.String(), "could not read Claude Code plugins") {
		t.Fatalf("expected plugin read warning, got stderr:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Skills (1)") {
		t.Fatalf("normal groups should still print, got:\n%s", stdout.String())
	}
}

func TestListPluginsSourcePluginNoPlugins(t *testing.T) {
	stdout, stderr := setupListPluginTest(t, nil)
	listCmd.Flags().Set("source", "plugin")

	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list --source plugin failed: %v", err)
	}

	if stdout.String() != "" {
		t.Fatalf("expected empty stdout, got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "No plugins found.") {
		t.Fatalf("expected No plugins found, got stderr:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "No items found.") {
		t.Fatalf("should not also print No items found, got stderr:\n%s", stderr.String())
	}
}
