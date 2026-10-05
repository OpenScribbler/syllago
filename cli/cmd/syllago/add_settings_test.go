package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// settingsAddEnv isolates HOME and the Library and writes settings to the
// project's .claude/settings.json. It returns the project root and Library.
func settingsAddEnv(t *testing.T, settings string) (projectRoot, globalDir string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	projectRoot, globalDir = t.TempDir(), t.TempDir()
	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })
	dir := filepath.Join(projectRoot, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	return projectRoot, globalDir
}

func hookDirs(t *testing.T, globalDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(globalDir, "hooks", "claude-code"))
	if err != nil {
		t.Fatalf("read hooks dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// A project hook added beside a global one of the same name is found again
// on the next add rather than copied to a new directory each time.
func TestAddSettings_CrossScopeRerunDoesNotDuplicate(t *testing.T) {
	projectRoot, globalDir := settingsAddEnv(t, settingsJSON)
	globalCopy := filepath.Join(globalDir, "hooks", "claude-code", "pre-bash-check")
	if err := os.MkdirAll(globalCopy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Save(globalCopy, &metadata.Meta{Name: "pre-bash-check", SourceScope: "global"}); err != nil {
		t.Fatal(err)
	}

	for run := 1; run <= 2; run++ {
		stdout, _ := output.SetForTest(t)
		if err := runAddSettings(catalog.Hooks, projectRoot, "claude-code", false, nil, false, "project", nil, "", "", ""); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if run == 2 && !strings.Contains(stdout.String(), "SKIP pre-bash-check-2") {
			t.Errorf("second run output = %s, want pre-bash-check-2 skipped", stdout.String())
		}
	}
	if got := strings.Join(hookDirs(t, globalDir), ","); got != "post-edit-check,pre-bash-check,pre-bash-check-2" {
		t.Errorf("hook dirs = %s, want post-edit-check,pre-bash-check,pre-bash-check-2", got)
	}
}

// Two handlers in one file that derive the same name are both added.
func TestAddSettings_SameNamedHooksBothAdded(t *testing.T) {
	projectRoot, globalDir := settingsAddEnv(t, `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo one", "statusMessage": "guard"}]},
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "echo two", "statusMessage": "guard"}]}
    ]
  }
}`)
	_, _ = output.SetForTest(t)
	if err := runAddSettings(catalog.Hooks, projectRoot, "claude-code", false, nil, false, "project", nil, "", "", ""); err != nil {
		t.Fatalf("runAddSettings: %v", err)
	}
	if got := strings.Join(hookDirs(t, globalDir), ","); got != "guard,guard-2" {
		t.Fatalf("hook dirs = %s, want guard,guard-2", got)
	}
	data, err := os.ReadFile(filepath.Join(globalDir, "hooks", "claude-code", "guard-2", "hook.json"))
	if err != nil || !strings.Contains(string(data), "echo two") {
		t.Errorf("guard-2 hook.json = %s (err %v), want the second handler", data, err)
	}
}

// An MCP server already in the Library is reported as in the Library.
func TestAddSettings_MCPDiscoveryShowsInLibrary(t *testing.T) {
	projectRoot, globalDir := settingsAddEnv(t, `{"mcpServers": {"db": {"command": "db-server"}}}`)
	_, _ = output.SetForTest(t)
	if err := runAddSettings(catalog.MCP, projectRoot, "claude-code", false, nil, false, "project", nil, "", "", ""); err != nil {
		t.Fatalf("runAddSettings: %v", err)
	}
	items := discoverSettingsForDisplay(projectRoot, "claude-code", nil, globalDir, catalog.MCP)
	if len(items) != 1 || items[0].Name != "db" {
		t.Fatalf("items = %+v, want db", items)
	}
	if items[0].Status != add.StatusInLibrary {
		t.Errorf("status = %v, want in library", items[0].Status)
	}
}

// Excluding a hook's derived name leaves out every handler that derives
// it; excluding a suffixed name leaves out that handler alone.
func TestAddSettings_ExcludeDerivedNameCoversSuffixes(t *testing.T) {
	const settings = `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo one", "statusMessage": "guard"}]},
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "echo two", "statusMessage": "guard"}]}
    ]
  }
}`
	for _, tc := range []struct{ exclude, want string }{
		{"guard", ""},
		{"guard-2", "guard"},
	} {
		t.Run(tc.exclude, func(t *testing.T) {
			projectRoot, globalDir := settingsAddEnv(t, settings)
			_, _ = output.SetForTest(t)
			if err := runAddSettings(catalog.Hooks, projectRoot, "claude-code", false, []string{tc.exclude}, false, "project", nil, "", "", ""); err != nil {
				t.Fatalf("runAddSettings: %v", err)
			}
			entries, _ := os.ReadDir(filepath.Join(globalDir, "hooks", "claude-code"))
			var got []string
			for _, e := range entries {
				got = append(got, e.Name())
			}
			if strings.Join(got, ",") != tc.want {
				t.Errorf("hook dirs = %v, want %q", got, tc.want)
			}
		})
	}
}

// A settings file that does not parse is named in a warning.
func TestAddSettings_UnparseableSettingsWarns(t *testing.T) {
	projectRoot, _ := settingsAddEnv(t, `{"hooks": `)
	_, stderr := output.SetForTest(t)
	if err := runAddSettings(catalog.Hooks, projectRoot, "claude-code", false, nil, false, "project", nil, "", "", ""); err != nil {
		t.Fatalf("runAddSettings: %v", err)
	}
	if !strings.Contains(stderr.String(), "Warning: skipped settings") || !strings.Contains(stderr.String(), "settings.json") {
		t.Errorf("stderr = %q, want a warning naming the settings file", stderr.String())
	}
}
