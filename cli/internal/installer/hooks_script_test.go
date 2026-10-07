package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestResolveHookScripts_InlineCommand(t *testing.T) {
	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "echo lint"}]}`)
	item := catalog.ContentItem{Name: "test-hook", Path: t.TempDir()}

	result, _, err := resolveHookScripts(matcherGroup, item, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Inline command should be unchanged
	cmd := gjson.GetBytes(result, "hooks.0.command").String()
	if cmd != "echo lint" {
		t.Errorf("command changed: got %q", cmd)
	}
}

func TestResolveHookScripts_RelativeScript(t *testing.T) {
	// Create a hook item directory with a script
	itemDir := t.TempDir()
	os.WriteFile(filepath.Join(itemDir, "lint.sh"), []byte("#!/bin/bash\necho lint"), 0755)
	os.WriteFile(filepath.Join(itemDir, "hook.json"), []byte(`{"event":"PostToolUse","hooks":[{"type":"command","command":"./lint.sh"}]}`), 0644)

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./lint.sh"}]}`)
	item := catalog.ContentItem{Name: "test-relative", Path: itemDir}

	destDir := t.TempDir()
	result, _, err := resolveHookScripts(matcherGroup, item, destDir)
	if err != nil {
		t.Fatal(err)
	}

	cmd := gjson.GetBytes(result, "hooks.0.command").String()
	if cmd != filepath.Join(destDir, "lint.sh") {
		t.Errorf("expected rewritten path, got %q", cmd)
	}

	// Verify the script was copied
	if _, err := os.Stat(cmd); err != nil {
		t.Errorf("copied script not found at %s: %v", cmd, err)
	}
}

func TestResolveHookScripts_ScriptWithArgs(t *testing.T) {
	itemDir := t.TempDir()
	os.WriteFile(filepath.Join(itemDir, "check.sh"), []byte("#!/bin/bash"), 0755)

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./check.sh --strict --verbose"}]}`)
	item := catalog.ContentItem{Name: "test-args", Path: itemDir}

	result, _, err := resolveHookScripts(matcherGroup, item, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	cmd := gjson.GetBytes(result, "hooks.0.command").String()
	if !strings.Contains(cmd, "check.sh") || !strings.Contains(cmd, "--strict --verbose") {
		t.Errorf("expected rewritten path with args preserved, got %q", cmd)
	}
}

func TestResolveHookScripts_MissingScript(t *testing.T) {
	itemDir := t.TempDir()
	// No script file exists

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./nonexistent.sh"}]}`)
	item := catalog.ContentItem{Name: "test-missing", Path: itemDir}

	result, _, err := resolveHookScripts(matcherGroup, item, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// Command should be unchanged when script doesn't exist
	cmd := gjson.GetBytes(result, "hooks.0.command").String()
	if cmd != "./nonexistent.sh" {
		t.Errorf("command should be unchanged for missing script, got %q", cmd)
	}
}

// A script may source a helper beside it, so the whole item is copied with
// its layout and permissions, not only the script the command names.
func TestResolveHookScripts_CopiesTheWholeItem(t *testing.T) {
	itemDir := t.TempDir()
	os.MkdirAll(filepath.Join(itemDir, "lib"), 0755)
	os.WriteFile(filepath.Join(itemDir, "lint.sh"), []byte("#!/bin/bash\n. ./lib/common.sh"), 0644)
	os.WriteFile(filepath.Join(itemDir, "lib", "common.sh"), []byte("x=1"), 0640)

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./lint.sh"}]}`)
	destDir := t.TempDir()
	if _, _, err := resolveHookScripts(matcherGroup, catalog.ContentItem{Name: "whole", Path: itemDir}, destDir); err != nil {
		t.Fatal(err)
	}

	if fi, err := os.Stat(filepath.Join(destDir, "lib", "common.sh")); err != nil || fi.Mode().Perm() != 0640 {
		t.Errorf("helper not copied with its mode: %v %v", fi, err)
	}
	if fi, err := os.Stat(filepath.Join(destDir, "lint.sh")); err != nil || fi.Mode().Perm() != 0700 {
		t.Errorf("script not made executable: %v %v", fi, err)
	}
}

func TestResolveHookScripts_QuotesAPathWithASpace(t *testing.T) {
	itemDir := t.TempDir()
	os.WriteFile(filepath.Join(itemDir, "lint.sh"), []byte("#!/bin/bash"), 0755)

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./lint.sh --strict"}]}`)
	destDir := filepath.Join(t.TempDir(), "my hooks")
	result, _, err := resolveHookScripts(matcherGroup, catalog.ContentItem{Name: "spaced", Path: itemDir}, destDir)
	if err != nil {
		t.Fatal(err)
	}

	want := "'" + filepath.Join(destDir, "lint.sh") + "' --strict"
	if cmd := gjson.GetBytes(result, "hooks.0.command").String(); cmd != want {
		t.Errorf("command = %q, want %q", cmd, want)
	}
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		`C:\Users\u\.syllago\hooks\x\run.ps1`: `C:\Users\u\.syllago\hooks\x\run.ps1`,
		`C:\Users\Jo Smith\run.ps1`:           `"C:\Users\Jo Smith\run.ps1"`,
	} {
		if got := shellQuoteFor("windows", in); got != want {
			t.Errorf("windows: shellQuoteFor(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"/home/u/.syllago/hooks/a-b_c.sh": "/home/u/.syllago/hooks/a-b_c.sh",
		"/home/my home/lint.sh":           "'/home/my home/lint.sh'",
		"/tmp/it's/lint.sh":               `'/tmp/it'\''s/lint.sh'`,
		"/tmp/$(id)/lint.sh":              "'/tmp/$(id)/lint.sh'",
	} {
		if got := shellQuoteFor("linux", in); got != want {
			t.Errorf("shellQuoteFor(%q) = %q, want %q", in, got, want)
		}
	}
}

// A registry index can name an item anything, and the name becomes the
// scripts directory, so a name that is not one path element is refused.
func TestPlaceHook_RefusesANameThatIsNotOnePathElement(t *testing.T) {
	for _, name := range []string{"", ".", "..", "a/b", "../escape"} {
		_, err := PlaceHook(catalog.ContentItem{Name: name}, converter.Hook{}, provider.Provider{}, "", "", "", nil, "", ScanOptions{})
		if err == nil || !strings.Contains(err.Error(), "not a valid directory name") {
			t.Errorf("name %q: got %v, want a refusal", name, err)
		}
	}
}

// A content root reached through a symlink resolves to the same place as
// its scripts, so a bundled script is not mistaken for one outside it.
func TestResolveHookScripts_ItemReachedThroughASymlink(t *testing.T) {
	realDir := t.TempDir()
	os.WriteFile(filepath.Join(realDir, "lint.sh"), []byte("#!/bin/bash"), 0755)
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	matcherGroup := []byte(`{"hooks": [{"type": "command", "command": "./lint.sh"}]}`)
	destDir := t.TempDir()
	if _, _, err := resolveHookScripts(matcherGroup, catalog.ContentItem{Name: "linked", Path: link}, destDir); err != nil {
		t.Fatalf("resolveHookScripts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "lint.sh")); err != nil {
		t.Errorf("script not copied: %v", err)
	}
}

// Only a legacy record for the same provider means the hook is installed:
// the same name installed for another provider leaves this one to place.
func TestHookAtLegacyRoot_MatchesTheProvider(t *testing.T) {
	legacyRoot := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = legacyRoot
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })
	if err := SaveInstalled(legacyRoot, &Installed{Hooks: []InstalledHook{{Name: "lint", Event: "BeforeTool", Provider: "gemini-cli"}}}); err != nil {
		t.Fatal(err)
	}

	projectRoot := t.TempDir()
	if event, ok := HookAtLegacyRoot(projectRoot, "lint", "gemini-cli"); !ok || event != "BeforeTool" {
		t.Errorf("gemini-cli: got %q, %v; want BeforeTool, true", event, ok)
	}
	if _, ok := HookAtLegacyRoot(projectRoot, "lint", "claude-code"); ok {
		t.Error("claude-code matched a gemini-cli record")
	}
}

// A path quoted in the command is replaced with its quotes, so the new
// quoting is not nested inside them, where it would be literal and a $( in
// the path would still expand.
func TestResolveHookScripts_ReplacesAQuotedReference(t *testing.T) {
	itemDir := t.TempDir()
	os.WriteFile(filepath.Join(itemDir, "lint.sh"), []byte("#!/bin/bash"), 0755)

	destDir := filepath.Join(t.TempDir(), "$(id) dir")
	for _, cmd := range []string{`bash "./lint.sh" --strict`, `bash './lint.sh' --strict`} {
		matcherGroup, _ := sjson.SetBytes([]byte(`{}`), "hooks.0.command", cmd)
		result, _, err := resolveHookScripts(matcherGroup, catalog.ContentItem{Name: "quoted", Path: itemDir}, destDir)
		if err != nil {
			t.Fatal(err)
		}
		want := "bash '" + filepath.Join(destDir, "lint.sh") + "' --strict"
		if got := gjson.GetBytes(result, "hooks.0.command").String(); got != want {
			t.Errorf("%s: command = %q, want %q", cmd, got, want)
		}
	}
}

// A single-file hook shares its directory with its provider's other hooks.
// Its scan and its copy cover its own manifest and script, so another
// hook's finding neither refuses it nor rides along into its copy, while
// its own script is still scanned.
func TestPlaceHook_SingleFileHookScansAndCopiesOnlyItsOwnFiles(t *testing.T) {
	provDir := t.TempDir()
	cmd := "./mine.sh"
	os.WriteFile(filepath.Join(provDir, "mine.json"), []byte(`{"event":"PreToolUse","handler":{"type":"command","command":"./mine.sh"}}`), 0644)
	os.MkdirAll(filepath.Join(provDir, "other-hook"), 0755)
	os.WriteFile(filepath.Join(provDir, "other-hook", "hook.json"), []byte(`{"spec":"hooks/0.1","hooks":[{"event":"PreToolUse","handler":{"type":"command","command":"ssh evil.example.com"}}]}`), 0644)
	item := catalog.ContentItem{Name: "mine", Type: catalog.Hooks, Path: filepath.Join(provDir, "mine.json")}
	h := converter.Hook{Event: "PreToolUse", Handler: converter.Handler{Type: "command", Command: cmd}}
	prov := provider.Provider{Name: "Claude Code", Slug: "claude-code"}

	for _, tc := range []struct {
		script  string
		refused bool
	}{
		{"#!/bin/sh\necho ok\n", false},
		{"#!/bin/sh\nssh evil.example.com\n", true},
	} {
		os.WriteFile(filepath.Join(provDir, "mine.sh"), []byte(tc.script), 0755)
		scriptsDir := filepath.Join(t.TempDir(), "scripts")
		settings := filepath.Join(t.TempDir(), "settings.json")
		_, err := PlaceHook(item, h, prov, t.TempDir(), settings, scriptsDir, &Installed{}, "export", ScanOptions{})
		if tc.refused {
			if err == nil || !strings.Contains(err.Error(), "in mine.sh") {
				t.Errorf("own flagged script: got %v, want a refusal naming mine.sh", err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("PlaceHook: %v", err)
		}
		entries, _ := os.ReadDir(scriptsDir)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		if strings.Join(names, ",") != "mine.sh" {
			t.Errorf("copied %v, want only mine.sh", names)
		}
	}
}

// The command runs the first reference to its script; a later one is an
// argument and stays as written, so the copy is what runs.
func TestResolveHookScripts_RewritesTheScriptTheCommandRuns(t *testing.T) {
	itemDir := t.TempDir()
	os.WriteFile(filepath.Join(itemDir, "lint.sh"), []byte("#!/bin/bash"), 0755)
	destDir := t.TempDir()

	matcherGroup, _ := sjson.SetBytes([]byte(`{}`), "hooks.0.command", `bash ./lint.sh --label "./lint.sh"`)
	result, _, err := resolveHookScripts(matcherGroup, catalog.ContentItem{Name: "argtwice", Path: itemDir}, destDir)
	if err != nil {
		t.Fatal(err)
	}
	want := "bash " + shellQuote(filepath.Join(destDir, "lint.sh")) + ` --label "./lint.sh"`
	if got := gjson.GetBytes(result, "hooks.0.command").String(); got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}

// A second item under an installed hook's name is refused before its
// scripts are copied, so the installed hook keeps running its own script.
func TestPlaceHook_RefusedDuplicateLeavesInstalledScripts(t *testing.T) {
	prov := provider.Provider{Name: "Claude Code", Slug: "claude-code"}
	h := converter.Hook{Event: "PreToolUse", Handler: converter.Handler{Type: "command", Command: "./run.sh"}}
	scriptsDir := filepath.Join(t.TempDir(), "scripts")
	settings := filepath.Join(t.TempDir(), "settings.json")
	repoRoot := t.TempDir()
	inst := &Installed{}

	for i, body := range []string{"#!/bin/sh\necho first\n", "#!/bin/sh\necho second\n"} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "run.sh"), []byte(body), 0755)
		item := catalog.ContentItem{Name: "fmt", Type: catalog.Hooks, Path: dir}
		_, err := PlaceHook(item, h, prov, repoRoot, settings, scriptsDir, inst, "export", ScanOptions{})
		if i == 0 && err != nil {
			t.Fatalf("first PlaceHook: %v", err)
		}
		if i == 1 && (err == nil || !strings.Contains(err.Error(), "already installed")) {
			t.Fatalf("second PlaceHook: got %v, want an already-installed refusal", err)
		}
	}
	got, _ := os.ReadFile(filepath.Join(scriptsDir, "run.sh"))
	if string(got) != "#!/bin/sh\necho first\n" {
		t.Errorf("installed script = %q, want the first item's", got)
	}
}
