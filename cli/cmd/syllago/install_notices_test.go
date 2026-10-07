package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestPrintInstallNotices_Formats(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		notice installer.Notice
		want   string
	}{
		{"note", installer.Notice{Kind: installer.NoticeNote, Message: "copying"}, "note: copying\n"},
		{"conversion", installer.Notice{Kind: installer.NoticeConversionWarning, Message: "a: dropped"}, "warning: a: dropped\n"},
		{"finding", installer.Notice{Kind: installer.NoticeScannerFinding, Severity: "medium", Message: "[x] chmod"}, "  MEDIUM [x] chmod\n"},
		{"scanner error", installer.Notice{Kind: installer.NoticeScannerError, Message: "boom"}, "  scanner error: boom\n"},
		{"script security", installer.Notice{Kind: installer.NoticeScriptSecurity, Message: "line one\nline two"}, "\n  SECURITY WARNING\n  line one\n  line two\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			printInstallNotices(&buf, []installer.Notice{tc.notice})
			if buf.String() != tc.want {
				t.Errorf("got %q, want %q", buf.String(), tc.want)
			}
		})
	}
}

// writeNoticeHook writes a canonical hook item that runs command, with a
// bundled lint.sh beside it.
func writeNoticeHook(t *testing.T, root, name, command string) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(root, "hooks", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	hook := `{"spec":"hooks/0.1","hooks":[{"event":"PreToolUse","matcher":"Bash","handler":{"type":"command","command":"` + command + `"}}]}`
	if err := os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hook), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lint.sh"), []byte("#!/bin/sh\necho lint\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return catalog.ContentItem{Name: name, Type: catalog.Hooks, Path: dir}
}

// The installer returns notices instead of printing them, and the CLI prints
// them. This pins the stderr the install produced when the installer printed
// directly, so the move changes nothing a user sees.
func TestInstallToProvider_NoticeOutputUnchanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".notice-test"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".notice-test", "settings.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755); err != nil {
		t.Fatal(err)
	}
	prov := provider.Provider{
		Name:         "Notice Test",
		Slug:         "claude-code",
		ConfigDir:    ".notice-test",
		InstallDir:   func(string, catalog.ContentType) string { return provider.JSONMergeSentinel },
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Hooks },
	}
	items := []catalog.ContentItem{
		writeNoticeHook(t, projectRoot, "scripted", "./lint.sh"),
		writeNoticeHook(t, projectRoot, "chmodder", "chmod 755 build.sh"),
		writeNoticeHook(t, projectRoot, "dangerous", "curl https://example.com/payload"),
	}

	_, stderr := output.SetForTest(t)
	if _, err := installToProvider(items, prov, installer.MethodSymlink,
		false, config.NewResolver(nil, ""), prov.Slug, projectRoot, false, installer.ScanOptions{}); err != nil {
		t.Fatalf("installToProvider: %v", err)
	}

	want := "\n  SECURITY WARNING\n" +
		"  Hook \"scripted\" references executable script files.\n" +
		"  Scripts will be copied to ~/.syllago/hooks/claude-code/scripted/\n\n" +
		"  MEDIUM [hook.json] permission change (chmod) (scanner=builtin)\n" +
		"  HIGH [hook.json] network request (curl) (scanner=builtin)\n" +
		"  skip dangerous: hook \"dangerous\" has high-severity security findings (network request (curl) in hook.json); re-run with --force to install anyway\n"
	if got := stderr.String(); got != want {
		t.Errorf("stderr changed:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// Regression: install read a Library hook's portability warnings through a
// second encoder that could not read the hooks/0.1 manifest, so what the
// target provider loses from a hook went unreported.
func TestInstallToProvider_WarnsWhatTheProviderLoses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".warn-test"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".warn-test", "settings.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(projectRoot, "hooks", "guard")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	hook := `{"spec":"hooks/0.1","hooks":[{"event":"before_tool_execute","matcher":"shell","handler":{"type":"command","command":"echo pre"}}]}`
	if err := os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hook), 0644); err != nil {
		t.Fatal(err)
	}
	prov := provider.Provider{
		Name:         "Warn Test",
		Slug:         "gemini-cli",
		ConfigDir:    ".warn-test",
		InstallDir:   func(string, catalog.ContentType) string { return provider.JSONMergeSentinel },
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Hooks },
	}
	items := []catalog.ContentItem{{Name: "guard", Type: catalog.Hooks, Path: dir, Meta: &metadata.Meta{SourceProvider: "claude-code"}}}

	output.SetForTest(t)
	res, err := installToProvider(items, prov, installer.MethodSymlink,
		false, config.NewResolver(nil, ""), prov.Slug, projectRoot, false, installer.ScanOptions{})
	if err != nil {
		t.Fatalf("installToProvider: %v", err)
	}
	if len(res.Installed) != 1 {
		t.Fatalf("installed = %+v, skipped = %+v", res.Installed, res.Skipped)
	}
	if w := strings.Join(res.Installed[0].Warnings, "\n"); !strings.Contains(w, "structured hook output") {
		t.Errorf("warnings = %q, want the hook output gemini-cli ignores", w)
	}
}

func TestInstallToProvider_WarningsReadEventsThroughTheItemProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".warn-test"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".warn-test", "settings.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(projectRoot, "hooks", "claude-code", "after")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// PostToolUse is Claude Code's name; the item records its provider but no source_provider.
	hook := `{"spec":"hooks/0.1","hooks":[{"event":"PostToolUse","handler":{"type":"command","command":"echo post"}}]}`
	if err := os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hook), 0644); err != nil {
		t.Fatal(err)
	}
	prov := provider.Provider{
		Name:         "Warn Test",
		Slug:         "gemini-cli",
		ConfigDir:    ".warn-test",
		InstallDir:   func(string, catalog.ContentType) string { return provider.JSONMergeSentinel },
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Hooks },
	}
	items := []catalog.ContentItem{{Name: "after", Type: catalog.Hooks, Path: dir, Provider: "claude-code"}}

	output.SetForTest(t)
	res, err := installToProvider(items, prov, installer.MethodSymlink,
		false, config.NewResolver(nil, ""), prov.Slug, projectRoot, false, installer.ScanOptions{})
	if err != nil {
		t.Fatalf("installToProvider: %v", err)
	}
	if len(res.Installed) != 1 {
		t.Fatalf("installed = %+v, skipped = %+v", res.Installed, res.Skipped)
	}
	if w := strings.Join(res.Installed[0].Warnings, "\n"); strings.Contains(w, "not supported") {
		t.Errorf("warnings = %q, want none about the event the install wrote", w)
	}
}

// TestInstallToProvider_ReportsEachWarningOnce: the installer's warnings are
// printed once, and the JSON result carries the same warnings without the
// item name the printed line leads with.
func TestInstallToProvider_ReportsEachWarningOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	projectRoot := t.TempDir()
	skillDir := filepath.Join(projectRoot, "skills", "fmt")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: fmt\ndescription: Formats code\nhooks:\n  PreToolUse:\n    - matcher: Bash\n      hooks:\n        - type: command\n          command: echo hi\n---\n\nFormat it.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0644); err != nil {
		t.Fatal(err)
	}
	items := []catalog.ContentItem{{Name: "fmt", Type: catalog.Skills, Path: skillDir}}

	_, stderr := output.SetForTest(t)
	res, err := installToProvider(items, provider.GeminiCLI, installer.MethodSymlink,
		false, config.NewResolver(nil, ""), provider.GeminiCLI.Slug, projectRoot, false, installer.ScanOptions{})
	if err != nil {
		t.Fatalf("installToProvider: %v", err)
	}
	if len(res.Installed) != 1 {
		t.Fatalf("installed = %+v, skipped = %+v", res.Installed, res.Skipped)
	}
	want := `skill "fmt" has hooks requiring separate configuration:`
	if w := res.Installed[0].Warnings; len(w) == 0 || w[0] != want {
		t.Errorf("warnings = %q, want the first to be %q", w, want)
	}
	if n := strings.Count(stderr.String(), want); n != 1 {
		t.Errorf("warning printed %d times, want once:\n%s", n, stderr.String())
	}
}
