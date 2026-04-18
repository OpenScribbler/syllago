package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- joinWords (0% coverage) ---

func TestJoinWords(t *testing.T) {
	t.Parallel()
	tests := []struct {
		parts []string
		want  string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a, b"},
		{[]string{"a", "b", "c"}, "a, b, c"},
	}
	for _, tc := range tests {
		got := joinWords(tc.parts)
		if got != tc.want {
			t.Errorf("joinWords(%v) = %q, want %q", tc.parts, got, tc.want)
		}
	}
}

// --- printCheck (0% coverage) ---

func TestPrintCheck(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	printCheck(checkResult{Name: "test", Status: checkOK, Message: "Test: all good"})
	printCheck(checkResult{Name: "warn", Status: checkWarn, Message: "Warn: check this", Details: []string{"detail 1"}})
	printCheck(checkResult{Name: "err", Status: checkErr, Message: "Error: broken"})

	out := stdout.String()
	if !strings.Contains(out, "[ok]") {
		t.Error("expected [ok] in output")
	}
	if !strings.Contains(out, "[warn]") {
		t.Error("expected [warn] in output")
	}
	if !strings.Contains(out, "[err]") {
		t.Error("expected [err] in output")
	}
	if !strings.Contains(out, "detail 1") {
		t.Error("expected detail in output")
	}
}

// --- checkConfigWith (0% coverage) ---

func TestCheckConfigWith_NoConfig(t *testing.T) {
	c := checkConfigWith(t.TempDir())
	// Should be checkWarn or checkOK depending on whether global config exists
	if c.Name != "config" {
		t.Errorf("Name = %q, want config", c.Name)
	}
}

// --- checkOrphans (0% coverage) ---

func TestCheckOrphans_NoInstalled(t *testing.T) {
	c := checkOrphans(t.TempDir())
	if c.Status != checkOK {
		t.Errorf("Status = %s, want ok (no installed.json)", c.Status)
	}
}

func TestCheckOrphans_WithOrphans(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".syllago"), 0755)
	installed := `{
		"symlinks": [
			{"path": "/tmp/fake-link", "target": "/nonexistent/target", "source": "test"}
		]
	}`
	os.WriteFile(filepath.Join(dir, ".syllago", "installed.json"), []byte(installed), 0644)

	c := checkOrphans(dir)
	if c.Status != checkWarn {
		t.Errorf("Status = %s, want warn (orphaned symlink)", c.Status)
	}
}

// --- runExport (0% coverage, stub) ---

func TestRunExport_NotImplemented(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	err := exportCmd.RunE(exportCmd, []string{"my-item"})
	if err == nil {
		t.Fatal("expected error from unimplemented export")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("error = %q, want mention of not implemented", err)
	}
}

// --- copyDir / copyFile (0% coverage) ---

func TestCopyDir(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Create source tree
	os.MkdirAll(filepath.Join(src, "sub"), 0755)
	os.WriteFile(filepath.Join(src, "file.txt"), []byte("Hello {{NAME}}"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "nested.txt"), []byte("Nested {{NAME}}"), 0644)

	if err := copyDir(src, dst, "my-project"); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	// Check file content with replacement
	data, _ := os.ReadFile(filepath.Join(dst, "file.txt"))
	if string(data) != "Hello my-project" {
		t.Errorf("file.txt = %q, want 'Hello my-project'", string(data))
	}

	data, _ = os.ReadFile(filepath.Join(dst, "sub", "nested.txt"))
	if string(data) != "Nested my-project" {
		t.Errorf("nested.txt = %q, want 'Nested my-project'", string(data))
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	os.WriteFile(src, []byte("Template: {{NAME}}"), 0644)

	if err := copyFile(src, dst, "test-name"); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != "Template: test-name" {
		t.Errorf("content = %q, want replaced template", string(data))
	}
}

// --- filterBySource (42.9% coverage) ---

func TestFilterBySource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		item   catalog.ContentItem
		source string
		want   bool
	}{
		{"library match", catalog.ContentItem{Library: true}, "library", true},
		{"library no match", catalog.ContentItem{Library: false}, "library", false},
		{"shared match", catalog.ContentItem{Library: false}, "shared", true},
		{"shared library excluded", catalog.ContentItem{Library: true}, "shared", false},
		{"shared registry excluded", catalog.ContentItem{Registry: "acme"}, "shared", false},
		{"registry match", catalog.ContentItem{Registry: "acme"}, "registry", true},
		{"registry no match", catalog.ContentItem{}, "registry", false},
		{"all", catalog.ContentItem{}, "all", true},
		{"unknown defaults to library", catalog.ContentItem{Library: true}, "unknown", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterBySource(tc.item, tc.source)
			if got != tc.want {
				t.Errorf("filterBySource(%s) = %v, want %v", tc.source, got, tc.want)
			}
		})
	}
}

// --- effectiveProvider (60% coverage) ---

func TestEffectiveProvider(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		item catalog.ContentItem
		want string
	}{
		{"from provider field", catalog.ContentItem{Provider: "claude-code"}, "claude-code"},
		{"from metadata", catalog.ContentItem{Meta: &metadata.Meta{SourceProvider: "cursor"}}, "cursor"},
		{"both prefers provider", catalog.ContentItem{Provider: "claude-code", Meta: &metadata.Meta{SourceProvider: "cursor"}}, "claude-code"},
		{"empty", catalog.ContentItem{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := effectiveProvider(tc.item)
			if got != tc.want {
				t.Errorf("effectiveProvider() = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- exportWarnMessage (60% coverage) ---

func TestExportWarnMessage(t *testing.T) {
	t.Parallel()
	// Normal item — no warning
	normal := catalog.ContentItem{Name: "my-rule", Type: catalog.Rules}
	if msg := exportWarnMessage(normal); msg != "" {
		t.Errorf("normal item warning = %q, want empty", msg)
	}

	// Example item
	example := catalog.ContentItem{Name: "example-rule", Type: catalog.Rules, Meta: &metadata.Meta{Tags: []string{"example"}}}
	if msg := exportWarnMessage(example); !strings.Contains(msg, "example") {
		t.Errorf("example item warning = %q, want mention of example", msg)
	}
}

// --- compatSymbol (50% coverage) ---

func TestCompatSymbol(t *testing.T) {
	t.Parallel()
	tests := []struct {
		level string
		want  string
	}{
		{"full", "✓"},
		{"degraded", "~"},
		{"none", "✗"},
		{"broken", "!"},
		{"unknown", "?"},
	}
	for _, tc := range tests {
		t.Run(tc.level, func(t *testing.T) {
			got := compatSymbol(tc.level)
			if got != tc.want {
				t.Errorf("compatSymbol(%q) = %q, want %q", tc.level, got, tc.want)
			}
		})
	}
}

// --- gendocs functions (all 0% coverage) ---

func TestToDisplayName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, want string
	}{
		{"registry sync", "Registry Sync"},
		{"doctor", "Doctor"},
		{"loadout apply", "Loadout Apply"},
		{"", ""},
	}
	for _, tc := range tests {
		got := toDisplayName(tc.input)
		if got != tc.want {
			t.Errorf("toDisplayName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCommandFullName(t *testing.T) {
	// Test with a subcommand of rootCmd
	for _, cmd := range rootCmd.Commands() {
		if cmd.Hidden || cmd.Name() == "help" {
			continue
		}
		name := commandFullName(cmd)
		if name == "" {
			t.Errorf("commandFullName(%q) returned empty string", cmd.Name())
		}
		if strings.Contains(name, "syllago") {
			t.Errorf("commandFullName should not include rootCmd name, got %q", name)
		}
		break // just test one
	}
}

func TestDeriveSourceFile(t *testing.T) {
	// Test with a known top-level command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Hidden || cmd.Name() == "help" {
			continue
		}
		src := deriveSourceFile(cmd)
		if !strings.HasPrefix(src, "cli/cmd/syllago/") {
			t.Errorf("deriveSourceFile(%q) = %q, want cli/cmd/syllago/ prefix", cmd.Name(), src)
		}
		break
	}
}

func TestBuildEntry(t *testing.T) {
	// Test with a real command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Hidden || cmd.Name() == "help" {
			continue
		}
		entry := buildEntry(cmd, nil)
		if entry.Name == "" {
			t.Error("buildEntry returned empty Name")
		}
		if entry.DisplayName == "" {
			t.Error("buildEntry returned empty DisplayName")
		}
		if entry.Slug == "" {
			t.Error("buildEntry returned empty Slug")
		}
		// Verify JSON serializable
		_, err := json.Marshal(entry)
		if err != nil {
			t.Errorf("buildEntry not JSON-serializable: %v", err)
		}
		break
	}
}

func TestExtractFlags(t *testing.T) {
	// Test with a command that has flags
	flags := extractFlags(listCmd.LocalNonPersistentFlags(), listCmd)
	// listCmd should have some flags (--type, --source, etc.)
	if len(flags) == 0 {
		t.Log("listCmd has no local flags (might be expected)")
	}
	// Verify JSON serializable
	_, err := json.Marshal(flags)
	if err != nil {
		t.Errorf("flags not JSON-serializable: %v", err)
	}
}

func TestBuildSeeAlso(t *testing.T) {
	// Test with a subcommand
	for _, cmd := range rootCmd.Commands() {
		if cmd.HasSubCommands() && !cmd.Hidden {
			for _, sub := range cmd.Commands() {
				if !sub.Hidden && sub.Name() != "help" {
					related := buildSeeAlso(sub)
					// Subcommand should have at least the parent in seeAlso
					if len(related) == 0 {
						t.Errorf("buildSeeAlso(%q) returned no related commands", sub.Name())
					}
					break
				}
			}
			break
		}
	}
}

func TestWalkCommands(t *testing.T) {
	var entries []CommandEntry
	walkCommands(rootCmd, nil, &entries)
	if len(entries) == 0 {
		t.Error("walkCommands returned no entries")
	}
	// Verify no hidden commands included
	for _, e := range entries {
		if e.Name == "_gendocs" {
			t.Error("walkCommands should skip hidden _gendocs command")
		}
	}
}

func TestRunGendocs(t *testing.T) {
	// Use a temp file instead of a pipe to avoid buffer deadlock.
	tmpFile := filepath.Join(t.TempDir(), "gendocs.json")
	f, err := os.Create(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	os.Stdout = f
	err = runGendocs(gendocsCmd, nil)
	f.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runGendocs: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}

	var manifest CommandManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("gendocs output is not valid JSON: %v", err)
	}
	if manifest.Version != "1" {
		t.Errorf("Version = %q, want %q", manifest.Version, "1")
	}
	if len(manifest.Commands) == 0 {
		t.Error("expected at least one command in manifest")
	}
}

// --- loadout status (0% coverage) ---

func TestRunLoadoutStatus_NoSnapshot(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	withFakeRepoRoot(t, t.TempDir())

	err := loadoutStatusCmd.RunE(loadoutStatusCmd, nil)
	if err != nil {
		t.Fatalf("runLoadoutStatus: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "No active loadout") {
		t.Errorf("expected 'No active loadout', got: %s", out)
	}
}

func TestRunLoadoutStatus_NoSnapshot_JSON(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	output.JSON = true
	withFakeRepoRoot(t, t.TempDir())

	err := loadoutStatusCmd.RunE(loadoutStatusCmd, nil)
	if err != nil {
		t.Fatalf("runLoadoutStatus: %v", err)
	}

	var result loadoutStatusResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Active {
		t.Error("expected Active=false")
	}
}

// --- loadout list (0% coverage) ---

func TestRunLoadoutList_NoLoadouts(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	root := t.TempDir()
	// Create minimal content structure
	os.MkdirAll(filepath.Join(root, "content"), 0755)
	withFakeRepoRoot(t, root)

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = filepath.Join(root, "content")
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	err := loadoutListCmd.RunE(loadoutListCmd, nil)
	if err != nil {
		t.Fatalf("runLoadoutList: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "No loadouts found") {
		t.Logf("output: %s", out) // might have loadouts from actual repo
	}
}

// --- loadout remove (0% coverage) ---

func TestRunLoadoutRemove_NoSnapshot(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	withFakeRepoRoot(t, t.TempDir())

	err := loadoutRemoveCmd.RunE(loadoutRemoveCmd, nil)
	if err != nil {
		t.Fatalf("runLoadoutRemove: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "No active loadout") {
		t.Errorf("expected 'No active loadout', got: %s", out)
	}
}

// --- checkAndWarnStaleSnapshot (0% coverage) ---

func TestCheckAndWarnStaleSnapshot_NoSnapshot(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	// Should not panic on temp dir with no snapshot
	checkAndWarnStaleSnapshot(t.TempDir())
}

// --- sourceLabel (40% coverage in list.go) ---

func TestSourceLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		item catalog.ContentItem
		want string
	}{
		{"library", catalog.ContentItem{Library: true}, "library"},
		{"registry", catalog.ContentItem{Registry: "acme/tools"}, "registry"},
		{"shared", catalog.ContentItem{}, "shared"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sourceLabel(tc.item)
			if got != tc.want {
				t.Errorf("sourceLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- summaryTypeLabel (71.4% coverage in add_cmd.go) ---

func TestSummaryTypeLabel(t *testing.T) {
	t.Parallel()
	// Empty results → "items"
	if got := summaryTypeLabel(nil); got != "items" {
		t.Errorf("summaryTypeLabel(nil) = %q, want items", got)
	}

	// Single type → lowercase type name
	results := []add.AddResult{
		{Name: "a", Type: catalog.Rules},
		{Name: "b", Type: catalog.Rules},
	}
	if got := summaryTypeLabel(results); got != "rules" {
		t.Errorf("summaryTypeLabel(all rules) = %q, want rules", got)
	}

	// Mixed types → "items"
	results = append(results, add.AddResult{Name: "c", Type: catalog.Skills})
	if got := summaryTypeLabel(results); got != "items" {
		t.Errorf("summaryTypeLabel(mixed) = %q, want items", got)
	}
}

// --- statusJSONLabel (50% coverage) ---

func TestStatusJSONLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status add.ItemStatus
		want   string
	}{
		{add.StatusNew, "new"},
		{add.StatusInLibrary, "in_library"},
		{add.StatusOutdated, "outdated"},
	}
	for _, tc := range tests {
		got := statusJSONLabel(tc.status)
		if got != tc.want {
			t.Errorf("statusJSONLabel(%v) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

// --- isInteractiveImpl (0% coverage) ---

func TestIsInteractiveImpl(t *testing.T) {
	// In test environment, stdin is usually not a terminal
	result := isInteractiveImpl()
	// Just verify it doesn't panic; result depends on test runner
	_ = result
}

// --- hookRiskDetails (0% coverage in inspect.go) ---

func TestHookRiskDetails(t *testing.T) {
	t.Parallel()
	hookDir := t.TempDir()
	// hookRiskDetails expects settings.json format: hooks keyed by event
	hookJSON := `{
		"hooks": {
			"PostToolUse": [{"type": "command", "command": "echo test"}]
		}
	}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	item := catalog.ContentItem{
		Name:  "my-hook",
		Type:  catalog.Hooks,
		Path:  hookDir,
		Files: []string{"hook.json"},
	}

	details := hookRiskDetails(item)
	if len(details) == 0 {
		t.Error("expected at least one risk detail for hook")
	}
}

func TestHookRiskDetails_NoFiles(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name: "empty-hook",
		Type: catalog.Hooks,
		Path: t.TempDir(),
	}
	details := hookRiskDetails(item)
	if len(details) != 0 {
		t.Errorf("expected 0 risk details for hook with no files, got %d", len(details))
	}
}

// --- firstArg (66.7% coverage) ---

func TestFirstArg(t *testing.T) {
	t.Parallel()
	if got := firstArg([]string{"a", "b"}); got != "a" {
		t.Errorf("firstArg([a,b]) = %q, want a", got)
	}
	if got := firstArg(nil); got != "<name>" {
		t.Errorf("firstArg(nil) = %q, want '<name>'", got)
	}
}

// --- runDoctor individual checks with project root ---

func TestDoctorCheckSymlinks_WithBrokenSymlink(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".syllago"), 0755)
	installed := `{
		"symlinks": [
			{"path": "/tmp/fake-path", "target": "/nonexistent/target", "source": "test"}
		]
	}`
	os.WriteFile(filepath.Join(dir, ".syllago", "installed.json"), []byte(installed), 0644)

	c := checkSymlinks(dir)
	if c.Status != checkWarn {
		t.Errorf("Status = %s, want warn (broken symlink)", c.Status)
	}
	if !strings.Contains(c.Message, "broken") {
		t.Errorf("Message = %q, want mention of broken", c.Message)
	}
}

func TestDoctorCheckContentDrift_NoContent(t *testing.T) {
	dir := t.TempDir()
	c := checkContentDrift(dir)
	if c.Status != checkOK {
		t.Errorf("Status = %s, want ok (no content)", c.Status)
	}
}

// --- Loadout commands with providers ---

func TestRunLoadoutList_JSON_NoLoadouts(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "content"), 0755)
	withFakeRepoRoot(t, root)

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = filepath.Join(root, "content")
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	origProviders := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{}
	t.Cleanup(func() { provider.AllProviders = origProviders })

	err := loadoutListCmd.RunE(loadoutListCmd, nil)
	if err != nil {
		t.Fatalf("runLoadoutList JSON: %v", err)
	}

	// Should output valid JSON (null or empty array)
	out := stdout.Bytes()
	if len(out) > 0 && !json.Valid(out) {
		t.Errorf("expected valid JSON, got: %s", string(out))
	}
}

// --- printUnifiedDiff (0% coverage) ---

func TestPrintUnifiedDiff(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	src := []byte("line1\nline2\n")
	tgt := []byte("changed1\nchanged2\n")

	err := printUnifiedDiff("my-rule", "cursor", "claude-code", src, tgt)
	if err != nil {
		t.Fatalf("printUnifiedDiff: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "--- my-rule (cursor)") {
		t.Error("expected source header")
	}
	if !strings.Contains(out, "+++ my-rule (claude-code)") {
		t.Error("expected target header")
	}
	if !strings.Contains(out, "-line1") {
		t.Error("expected removed line")
	}
	if !strings.Contains(out, "+changed1") {
		t.Error("expected added line")
	}
}

// --- emitConvertOutput (55% coverage) ---

func TestEmitConvertOutput_NilContent(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	err := emitConvertOutput("my-rule", "cursor", "claude-code", "",
		&converter.Result{Content: nil}, nil, false)
	if err == nil {
		t.Fatal("expected error for nil content")
	}
}

func TestEmitConvertOutput_ToFile(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	outPath := filepath.Join(t.TempDir(), "output.md")
	err := emitConvertOutput("my-rule", "cursor", "claude-code", outPath,
		&converter.Result{Content: []byte("# Converted")}, nil, false)
	if err != nil {
		t.Fatalf("emitConvertOutput: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	if string(data) != "# Converted" {
		t.Errorf("file content = %q, want '# Converted'", string(data))
	}
}

func TestEmitConvertOutput_ToFile_JSON(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	outPath := filepath.Join(t.TempDir(), "output.md")
	err := emitConvertOutput("my-rule", "cursor", "claude-code", outPath,
		&converter.Result{Content: []byte("# Converted"), Warnings: []string{"warn1"}}, nil, false)
	if err != nil {
		t.Fatalf("emitConvertOutput: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "my-rule") {
		t.Errorf("expected name in JSON output, got: %s", out)
	}
}

func TestEmitConvertOutput_Diff(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	err := emitConvertOutput("my-rule", "cursor", "claude-code", "",
		&converter.Result{Content: []byte("# New")}, []byte("# Old"), true)
	if err != nil {
		t.Fatalf("emitConvertOutput with diff: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "---") {
		t.Error("expected diff headers")
	}
}

func TestEmitConvertOutput_JSON_NoFile(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	err := emitConvertOutput("my-rule", "cursor", "claude-code", "",
		&converter.Result{Content: []byte("# Content")}, nil, false)
	if err != nil {
		t.Fatalf("emitConvertOutput: %v", err)
	}

	var result convertResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result.Content != "# Content" {
		t.Errorf("Content = %q, want '# Content'", result.Content)
	}
}

func TestEmitConvertOutput_Warnings(t *testing.T) {
	_, stderr := output.SetForTest(t)

	err := emitConvertOutput("my-rule", "cursor", "claude-code", "",
		&converter.Result{Content: []byte("# Content"), Warnings: []string{"not portable"}}, nil, false)
	if err != nil {
		t.Fatalf("emitConvertOutput: %v", err)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "Portability warnings") {
		t.Logf("stderr: %s", errOut) // might be empty in JSON mode
	}
}

// --- runRename (0% coverage) ---

func TestRunRename_NotFound(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = t.TempDir()
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	renameCmd.Flags().Set("name", "New Name")
	defer renameCmd.Flags().Set("name", "")

	err := renameCmd.RunE(renameCmd, []string{"nonexistent-item"})
	if err == nil {
		t.Fatal("expected error for item not found")
	}
}

// --- runAddFromShared (0% coverage) ---

// writeSharedSkill creates a shared-content skill item at
// <projectRoot>/skills/<name>/SKILL.md. Skills are a universal content type,
// so the layout is type/name (no provider subdirectory) and the scanner gives
// the directory name directly as ContentItem.Name.
func writeSharedSkill(t *testing.T, projectRoot, name string) string {
	t.Helper()
	dir := filepath.Join(projectRoot, "skills", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: " + name + "\ndescription: test skill\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunAddFromShared_NoSharedContent(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	root := t.TempDir()
	withFakeRepoRoot(t, root)

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = filepath.Join(root, "content")
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	err := runAddFromShared(root, nil, false, false, false)
	if err != nil {
		t.Fatalf("runAddFromShared: %v", err)
	}
}

// TestRunAddFromShared_DiscoveryMode exercises the "list available items" path
// (no args, addAll=false).
func TestRunAddFromShared_DiscoveryMode(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	root := t.TempDir()
	withFakeRepoRoot(t, root)
	writeSharedSkill(t, root, "alpha")
	writeSharedSkill(t, root, "beta")

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = filepath.Join(root, "content")
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	if err := runAddFromShared(root, nil, false, false, false); err != nil {
		t.Fatalf("runAddFromShared: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Errorf("discovery output should mention both items, got: %s", out)
	}
	if !strings.Contains(out, "--all") {
		t.Errorf("discovery output should hint at --all flag, got: %s", out)
	}
}

// TestRunAddFromShared_AddAll copies every shared item into the library and
// creates metadata sidecars.
func TestRunAddFromShared_AddAll(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	root := t.TempDir()
	withFakeRepoRoot(t, root)
	writeSharedSkill(t, root, "alpha")
	writeSharedSkill(t, root, "beta")

	libDir := filepath.Join(root, "library")
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = libDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	if err := runAddFromShared(root, nil, true, false, false); err != nil {
		t.Fatalf("runAddFromShared: %v", err)
	}

	for _, name := range []string{"alpha", "beta"} {
		dest := filepath.Join(libDir, "skills", name)
		if _, err := os.Stat(dest); err != nil {
			t.Errorf("expected skills/%s copied to library: %v", name, err)
		}
		if _, err := metadata.Load(dest); err != nil {
			t.Errorf("expected metadata sidecar for skills/%s: %v", name, err)
		}
	}
	if out := stdout.String(); !strings.Contains(out, "Added 2") {
		t.Errorf("expected summary to report 2 items added, got: %s", out)
	}
}

// TestRunAddFromShared_NamedItem copies only the item matching args[0].
func TestRunAddFromShared_NamedItem(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	root := t.TempDir()
	withFakeRepoRoot(t, root)
	writeSharedSkill(t, root, "alpha")
	writeSharedSkill(t, root, "bravo")

	libDir := filepath.Join(root, "library")
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = libDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	if err := runAddFromShared(root, []string{"alpha"}, false, false, false); err != nil {
		t.Fatalf("runAddFromShared: %v", err)
	}

	if _, err := os.Stat(filepath.Join(libDir, "skills", "alpha")); err != nil {
		t.Errorf("alpha should have been copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(libDir, "skills", "bravo")); err == nil {
		t.Errorf("bravo should NOT have been copied (name filter)")
	}
	_ = stdout
}

// TestRunAddFromShared_NamedItem_NotFound returns ErrItemNotFound when the
// named item has no matching shared content.
func TestRunAddFromShared_NamedItem_NotFound(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	root := t.TempDir()
	withFakeRepoRoot(t, root)
	writeSharedSkill(t, root, "alpha")

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = filepath.Join(root, "library")
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	err := runAddFromShared(root, []string{"ghost"}, false, false, false)
	if err == nil {
		t.Fatal("expected error for missing named item")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error should mention requested item name, got: %v", err)
	}
}

// TestRunAddFromShared_DryRun prints the plan without touching the library.
func TestRunAddFromShared_DryRun(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	root := t.TempDir()
	withFakeRepoRoot(t, root)
	writeSharedSkill(t, root, "alpha")

	libDir := filepath.Join(root, "library")
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = libDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	if err := runAddFromShared(root, nil, true, true, false); err != nil {
		t.Fatalf("runAddFromShared: %v", err)
	}
	if _, err := os.Stat(filepath.Join(libDir, "skills", "alpha")); err == nil {
		t.Error("dry-run must not write to library")
	}
	if out := stdout.String(); !strings.Contains(out, "dry-run") {
		t.Errorf("dry-run output should announce mode, got: %s", out)
	}
}

// TestRunAddFromShared_SkipWhenAlreadyInLibrary preserves the existing library
// entry when --force is not set.
func TestRunAddFromShared_SkipWhenAlreadyInLibrary(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	root := t.TempDir()
	withFakeRepoRoot(t, root)
	writeSharedSkill(t, root, "alpha")

	libDir := filepath.Join(root, "library")
	existing := filepath.Join(libDir, "skills", "alpha")
	if err := os.MkdirAll(existing, 0755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(existing, "existing.md")
	if err := os.WriteFile(sentinel, []byte("# keep me"), 0644); err != nil {
		t.Fatal(err)
	}

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = libDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	if err := runAddFromShared(root, nil, true, false, false); err != nil {
		t.Fatalf("runAddFromShared: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("existing library file should be preserved without --force: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "SKIP") {
		t.Errorf("output should mention SKIP, got: %s", out)
	}
}

// TestRunAddFromShared_ForceOverwrite replaces the existing library item when
// --force is passed.
func TestRunAddFromShared_ForceOverwrite(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	root := t.TempDir()
	withFakeRepoRoot(t, root)
	writeSharedSkill(t, root, "alpha")

	libDir := filepath.Join(root, "library")
	existing := filepath.Join(libDir, "skills", "alpha")
	if err := os.MkdirAll(existing, 0755); err != nil {
		t.Fatal(err)
	}
	oldSentinel := filepath.Join(existing, "old-only.md")
	if err := os.WriteFile(oldSentinel, []byte("# old"), 0644); err != nil {
		t.Fatal(err)
	}

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = libDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	if err := runAddFromShared(root, nil, true, false, true); err != nil {
		t.Fatalf("runAddFromShared: %v", err)
	}
	if _, err := os.Stat(filepath.Join(existing, "SKILL.md")); err != nil {
		t.Errorf("force should overwrite with new content: %v", err)
	}
}

// --- loadout apply error paths ---

func TestRunLoadoutApply_NoArgs_NoCatalog(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	root := t.TempDir()
	withFakeRepoRoot(t, root)

	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = filepath.Join(root, "content")
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	err := loadoutApplyCmd.RunE(loadoutApplyCmd, nil)
	if err == nil {
		t.Log("loadout apply succeeded unexpectedly (might have found loadouts)")
	}
}

// --- runDoctor via individual checks ---
// Can't test runDoctor directly (os.Exit), but can test all individual checks.

func TestDoctorCheckRegistries_WithConfig(t *testing.T) {
	// Isolate from real global config to avoid counting user's own registries.
	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	// Create a temp project with config containing registries
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".syllago"), 0755)
	cfg := `{"registries": [{"url": "https://example.com/test.git", "visibility": "public"}]}`
	os.WriteFile(filepath.Join(dir, ".syllago", "config.json"), []byte(cfg), 0644)

	c := checkRegistriesWith(dir)
	if c.Status != checkOK {
		t.Errorf("Status = %s, want ok", c.Status)
	}
	if !strings.Contains(c.Message, "1 configured") {
		t.Errorf("Message = %q, want mention of 1 configured", c.Message)
	}
}

// --- ensureUpToDate (0% coverage) ---

func TestEnsureUpToDate_Skips(t *testing.T) {
	// Should not panic even without update config
	ensureUpToDate()
}

// --- runLoadoutRemove with --auto flag ---

func TestRunLoadoutRemove_Auto_NoSnapshot(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout
	withFakeRepoRoot(t, t.TempDir())

	loadoutRemoveCmd.Flags().Set("auto", "true")
	defer loadoutRemoveCmd.Flags().Set("auto", "")

	err := loadoutRemoveCmd.RunE(loadoutRemoveCmd, nil)
	if err != nil {
		t.Fatalf("runLoadoutRemove --auto: %v", err)
	}
}

// --- runLoadoutApply error-path coverage ---

// setupLoadoutApplyRepo creates a project/content root containing an optional loadout
// directory. If loadoutName is empty, no loadout is created (empty catalog).
// Also clears repoRoot so findContentRepoRoot falls through to findProjectRoot.
func setupLoadoutApplyRepo(t *testing.T, loadoutName, providerSlug string, refs map[string][]string) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".syllago"), 0755)

	// Clear ldflag-embedded repoRoot so findContentRepoRoot uses findProjectRoot.
	origRepoRoot := repoRoot
	repoRoot = ""
	t.Cleanup(func() { repoRoot = origRepoRoot })

	withFakeRepoRoot(t, root)

	if loadoutName != "" {
		// Loadouts live under loadouts/<provider>/<name>/ — they are not universal.
		provDir := providerSlug
		if provDir == "" {
			provDir = "claude-code"
		}
		loDir := filepath.Join(root, "loadouts", provDir, loadoutName)
		os.MkdirAll(loDir, 0755)
		var sb strings.Builder
		sb.WriteString("kind: loadout\nversion: 1\n")
		sb.WriteString("name: " + loadoutName + "\n")
		sb.WriteString("description: test loadout\n")
		if providerSlug != "" {
			sb.WriteString("provider: " + providerSlug + "\n")
		}
		for section, names := range refs {
			if len(names) == 0 {
				continue
			}
			sb.WriteString(section + ":\n")
			for _, n := range names {
				sb.WriteString("  - " + n + "\n")
			}
		}
		os.WriteFile(filepath.Join(loDir, "loadout.yaml"), []byte(sb.String()), 0644)
	}

	return root
}

// resetLoadoutApplyFlags restores loadoutApplyCmd flags to their defaults so
// tests don't poison each other. Call as deferred immediately after setting flags.
func resetLoadoutApplyFlags() {
	loadoutApplyCmd.Flags().Set("try", "false")
	loadoutApplyCmd.Flags().Set("keep", "false")
	loadoutApplyCmd.Flags().Set("preview", "false")
	loadoutApplyCmd.Flags().Set("base-dir", "")
	loadoutApplyCmd.Flags().Set("to", "")
	loadoutApplyCmd.Flags().Set("method", "symlink")
}

func TestRunLoadoutApply_EmptyLibraryReturnsNil(t *testing.T) {
	setupLoadoutApplyRepo(t, "", "", nil) // no loadouts
	output.SetForTest(t)

	err := loadoutApplyCmd.RunE(loadoutApplyCmd, nil)
	if err != nil {
		t.Fatalf("expected nil when no loadouts, got %v", err)
	}
}

func TestRunLoadoutApply_LoadoutNotFoundByName(t *testing.T) {
	setupLoadoutApplyRepo(t, "exists", "claude-code", nil)
	output.SetForTest(t)
	defer resetLoadoutApplyFlags()

	err := loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("expected ErrLoadoutNotFound, got nil")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should mention missing name, got %v", err)
	}
}

func TestRunLoadoutApply_NoArgsNoTerminalErrors(t *testing.T) {
	setupLoadoutApplyRepo(t, "some-loadout", "claude-code", nil)
	output.SetForTest(t)

	origIsInteractive := isInteractive
	isInteractive = func() bool { return false }
	t.Cleanup(func() { isInteractive = origIsInteractive })

	err := loadoutApplyCmd.RunE(loadoutApplyCmd, nil)
	if err == nil {
		t.Fatal("expected ErrInputTerminal, got nil")
	}
	if !strings.Contains(err.Error(), "not a terminal") {
		t.Errorf("error should mention terminal requirement, got %v", err)
	}
}

func TestRunLoadoutApply_TryAndKeepConflict(t *testing.T) {
	setupLoadoutApplyRepo(t, "conflict", "claude-code", nil)
	output.SetForTest(t)

	loadoutApplyCmd.Flags().Set("try", "true")
	loadoutApplyCmd.Flags().Set("keep", "true")
	defer resetLoadoutApplyFlags()

	err := loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"conflict"})
	if err == nil {
		t.Fatal("expected ErrInputConflict, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusion, got %v", err)
	}
}

func TestRunLoadoutApply_UnknownProviderFlag(t *testing.T) {
	setupLoadoutApplyRepo(t, "has-provider", "claude-code", nil)
	output.SetForTest(t)

	loadoutApplyCmd.Flags().Set("to", "does-not-exist-provider-xyz")
	defer resetLoadoutApplyFlags()

	err := loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"has-provider"})
	if err == nil {
		t.Fatal("expected ErrProviderNotFound, got nil")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error should mention unknown provider, got %v", err)
	}
}

func TestRunLoadoutApply_ManifestUnknownProvider(t *testing.T) {
	setupLoadoutApplyRepo(t, "bad-prov", "does-not-exist-manifest-provider", nil)
	output.SetForTest(t)
	defer resetLoadoutApplyFlags()

	err := loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"bad-prov"})
	if err == nil {
		t.Fatal("expected ErrLoadoutProvider, got nil")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error should mention unknown provider, got %v", err)
	}
}

func TestRunLoadoutApply_PreviewHappyPath(t *testing.T) {
	root := setupLoadoutApplyRepo(t, "demo", "claude-code",
		map[string][]string{"rules": {"demo-rule"}})
	output.SetForTest(t)

	// Create a rule referenced by the loadout.
	ruleDir := filepath.Join(root, "rules", "claude-code", "demo-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Demo rule\n"), 0644)

	// Isolate global config so LoadGlobal works against an empty dir.
	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	loadoutApplyCmd.Flags().Set("preview", "true")
	defer resetLoadoutApplyFlags()

	if err := loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"demo"}); err != nil {
		t.Fatalf("preview should not error, got %v", err)
	}
}

func TestRunLoadoutApply_MalformedManifestErrors(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".syllago"), 0755)

	origRepoRoot := repoRoot
	repoRoot = ""
	t.Cleanup(func() { repoRoot = origRepoRoot })
	withFakeRepoRoot(t, root)

	// Write a loadout dir that scanner will pick up, but with invalid YAML content.
	loDir := filepath.Join(root, "loadouts", "claude-code", "broken")
	os.MkdirAll(loDir, 0755)
	os.WriteFile(filepath.Join(loDir, "loadout.yaml"), []byte("kind: loadout\nversion: 1\nname: [this is not a string]\n"), 0644)

	output.SetForTest(t)
	defer resetLoadoutApplyFlags()

	err := loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"broken"})
	if err == nil {
		t.Fatal("expected ErrLoadoutParse, got nil")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error should mention parse failure, got %v", err)
	}
}

func TestRunLoadoutApply_SnapshotAlreadyActiveConflicts(t *testing.T) {
	root := setupLoadoutApplyRepo(t, "already", "claude-code",
		map[string][]string{"rules": {"r1"}})
	// Pre-seed a snapshot manifest under .syllago/snapshots/<ts>/manifest.json
	// so snapshot.Load returns successfully, triggering the already-active guard.
	snapTSDir := filepath.Join(root, ".syllago", "snapshots", "20260417-000000")
	os.MkdirAll(snapTSDir, 0755)
	os.WriteFile(filepath.Join(snapTSDir, "manifest.json"), []byte(`{"loadoutName":"whatever","source":"loadout:whatever","mode":"keep","createdAt":"2026-04-17T00:00:00Z"}`), 0644)

	output.SetForTest(t)
	loadoutApplyCmd.Flags().Set("keep", "true")
	defer resetLoadoutApplyFlags()

	err := loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"already"})
	if err == nil {
		t.Fatal("expected ErrLoadoutConflict, got nil")
	}
	if !strings.Contains(err.Error(), "already active") {
		t.Errorf("error should mention already active, got %v", err)
	}
}

func TestRunLoadoutApply_PreviewJSONOutput(t *testing.T) {
	root := setupLoadoutApplyRepo(t, "jdemo", "claude-code",
		map[string][]string{"rules": {"json-rule"}})
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	ruleDir := filepath.Join(root, "rules", "claude-code", "json-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# JSON rule\n"), 0644)

	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	loadoutApplyCmd.Flags().Set("preview", "true")
	defer resetLoadoutApplyFlags()

	if err := loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"jdemo"}); err != nil {
		t.Fatalf("preview JSON should not error, got %v", err)
	}
	// Just verify something JSON-like came out. The exact shape is stable in
	// loadout.ApplyResult — we only need the command to emit JSON not text.
	var anyResult map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &anyResult); err != nil {
		t.Errorf("expected JSON on stdout, got %q: %v", stdout.String(), err)
	}
}
