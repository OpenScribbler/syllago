package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/output"
)

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		valid   bool
	}{
		{"simple semver", "1.0.0", true},
		{"with prerelease", "1.0.0-alpha", true},
		{"with prerelease and build", "1.0.0-alpha.1+build.123", true},
		{"patch version", "0.0.1", true},
		{"major version", "2.0.0", true},
		{"empty string", "", false},
		{"missing patch", "1.0", false},
		{"with v prefix", "v1.0.0", false},
		{"with spaces", "1.0.0 ", false},
		{"injection attempt", "1.0.0 -X main.repoRoot=/tmp/evil", false},
		{"special chars", "1.0.0; rm -rf /", false},
		{"newline injection", "1.0.0\n-X main.evil=true", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVersion(tt.version)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected invalid, got nil error")
			}
		})
	}
}

func TestRootCommandHelp(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "nesco") {
		t.Error("help output should contain 'nesco'")
	}
	if !strings.Contains(out, "init") || !strings.Contains(out, "version") {
		t.Error("help output should list subcommands")
	}
}

func TestHelpTextMentionsTUI(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--help"})
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	}()

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "interactive") && !strings.Contains(out, "TUI") {
		t.Error("help text should mention interactive/TUI mode")
	}

	if !strings.Contains(out, "without arguments") && !strings.Contains(out, "no arguments") {
		t.Error("help text should explain running without arguments")
	}
}

func TestVersionCommand(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}

func TestNoColorFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{
			name: "with --no-color flag",
			args: []string{"--no-color", "--help"},
		},
		{
			name: "with NO_COLOR env var",
			args: []string{"--help"},
			env:  map[string]string{"NO_COLOR": "1"},
		},
		{
			name: "with TERM=dumb",
			args: []string{"--help"},
			env:  map[string]string{"TERM": "dumb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				old := os.Getenv(k)
				os.Setenv(k, v)
				defer os.Setenv(k, old)
			}

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetArgs(tt.args)
			defer func() {
				rootCmd.SetOut(nil)
				rootCmd.SetArgs(nil)
			}()

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}

			// When color output is added in Phase 3, add ANSI code checks here.
			// For now, verify the flag is wired and doesn't break execution.
		})
	}
}

func TestQuietFlag(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantQuiet bool
	}{
		{
			name:      "version without quiet",
			args:      []string{"version"},
			wantQuiet: false,
		},
		{
			name:      "version with --quiet",
			args:      []string{"--quiet", "version"},
			wantQuiet: true,
		},
		{
			name:      "version with -q",
			args:      []string{"-q", "version"},
			wantQuiet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origQuiet := output.Quiet
			defer func() { output.Quiet = origQuiet }()

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetArgs(tt.args)
			defer func() {
				rootCmd.SetOut(nil)
				rootCmd.SetArgs(nil)
			}()

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output.Quiet != tt.wantQuiet {
				t.Errorf("output.Quiet = %v, want %v", output.Quiet, tt.wantQuiet)
			}
		})
	}
}

func TestVerboseFlag(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantVerbose bool
	}{
		{
			name:        "version without verbose",
			args:        []string{"version"},
			wantVerbose: false,
		},
		{
			name:        "version with --verbose",
			args:        []string{"--verbose", "version"},
			wantVerbose: true,
		},
		{
			name:        "version with -v",
			args:        []string{"-v", "version"},
			wantVerbose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origVerbose := output.Verbose
			defer func() { output.Verbose = origVerbose }()

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetArgs(tt.args)
			defer func() {
				rootCmd.SetOut(nil)
				rootCmd.SetArgs(nil)
			}()

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output.Verbose != tt.wantVerbose {
				t.Errorf("output.Verbose = %v, want %v", output.Verbose, tt.wantVerbose)
			}
		})
	}
}

func TestVersionCommandDevBuild(t *testing.T) {
	tests := []struct {
		name        string
		versionVar  string
		wantContain string
	}{
		{
			name:        "with version set",
			versionVar:  "1.2.3",
			wantContain: "1.2.3",
		},
		{
			name:        "dev build (empty version)",
			versionVar:  "",
			wantContain: "(dev build)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldVersion := version
			version = tt.versionVar
			defer func() { version = oldVersion }()

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetArgs([]string{"version"})
			defer func() {
				rootCmd.SetOut(nil)
				rootCmd.SetArgs(nil)
			}()

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("version command failed: %v", err)
			}

			out := buf.String()
			if !strings.Contains(out, tt.wantContain) {
				t.Errorf("version output = %q, want to contain %q", out, tt.wantContain)
			}
		})
	}
}

func TestPrintExecuteError(t *testing.T) {
	var buf bytes.Buffer
	origWriter := output.ErrWriter
	output.ErrWriter = &buf
	defer func() { output.ErrWriter = origWriter }()

	// Normal error should be printed
	printExecuteError(fmt.Errorf("normal error"))
	if !strings.Contains(buf.String(), "normal error") {
		t.Error("normal error should be printed to stderr")
	}

	// Silent error should not be printed
	buf.Reset()
	printExecuteError(output.SilentError(fmt.Errorf("already shown")))
	if buf.Len() > 0 {
		t.Errorf("silent error should not print, got: %s", buf.String())
	}
}

func TestTUIErrorMessageContentRepoNotFound(t *testing.T) {
	oldFindProject := findProjectRoot
	findProjectRoot = func() (string, error) {
		return "", fmt.Errorf("no project root")
	}
	defer func() { findProjectRoot = oldFindProject }()

	oldRepoRoot := repoRoot
	repoRoot = ""
	defer func() { repoRoot = oldRepoRoot }()

	err := runTUI(rootCmd, []string{})
	if err == nil {
		t.Fatal("expected error when content repo not found")
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "skills/") {
		t.Error("error message should not mention internal 'skills/' directory")
	}
	if !strings.Contains(errMsg, "nesco") {
		t.Error("error message should mention 'nesco'")
	}
}

func TestWrapTTYError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantSubcmd   bool
		wantOriginal bool
	}{
		{
			name:       "TTY error is wrapped",
			err:        fmt.Errorf("could not open a new TTY: something low-level"),
			wantSubcmd: true,
		},
		{
			name:         "non-TTY error passes through",
			err:          fmt.Errorf("normal error"),
			wantOriginal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := wrapTTYError(tt.err)
			msg := wrapped.Error()

			if tt.wantSubcmd && !strings.Contains(msg, "subcommand") {
				t.Errorf("TTY error should suggest subcommands, got: %s", msg)
			}
			if tt.wantOriginal && msg != tt.err.Error() {
				t.Errorf("non-TTY error should pass through unchanged, got: %s", msg)
			}
		})
	}

	// nil input should return nil
	if wrapTTYError(nil) != nil {
		t.Error("wrapTTYError(nil) should return nil")
	}
}

func TestHelpDocumentsExitCodes(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--help"})
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	}()

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Exit code") && !strings.Contains(out, "exit code") {
		t.Error("help text should document exit codes")
	}

	for _, code := range []string{"0", "1", "2"} {
		if !strings.Contains(out, code) {
			t.Errorf("help text should mention exit code %s", code)
		}
	}
}

func TestGlobalFlags(t *testing.T) {
	flags := rootCmd.PersistentFlags()
	if flags.Lookup("json") == nil {
		t.Error("missing --json global flag")
	}
	if flags.Lookup("no-color") == nil {
		t.Error("missing --no-color global flag")
	}
	if flags.Lookup("quiet") == nil {
		t.Error("missing --quiet global flag")
	}
	if flags.Lookup("verbose") == nil {
		t.Error("missing --verbose global flag")
	}
}

func TestFindContentRepoRootConfigBased(t *testing.T) {
	tmp := t.TempDir()
	// Create project markers
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	// Create .nesco/config.json with contentRoot
	os.MkdirAll(filepath.Join(tmp, ".nesco"), 0755)
	os.WriteFile(filepath.Join(tmp, ".nesco", "config.json"),
		[]byte(`{"providers":[],"content_root":"content"}`), 0644)
	// Create the content dir so the path is valid
	os.MkdirAll(filepath.Join(tmp, "content"), 0755)

	oldFindProject := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = oldFindProject }()

	oldRepoRoot := repoRoot
	repoRoot = ""
	defer func() { repoRoot = oldRepoRoot }()

	got, err := findContentRepoRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(tmp, "content")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFindContentRepoRootContentDirFallback(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	// No config, but skills/ exists at project root
	os.MkdirAll(filepath.Join(tmp, "skills"), 0755)

	oldFindProject := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = oldFindProject }()

	oldRepoRoot := repoRoot
	repoRoot = ""
	defer func() { repoRoot = oldRepoRoot }()

	got, err := findContentRepoRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmp {
		t.Errorf("got %q, want %q", got, tmp)
	}
}

func TestFindContentRepoRootProjectRootFallback(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	// No config, no content dirs — should fall back to project root

	oldFindProject := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = oldFindProject }()

	oldRepoRoot := repoRoot
	repoRoot = ""
	defer func() { repoRoot = oldRepoRoot }()

	got, err := findContentRepoRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmp {
		t.Errorf("got %q, want %q", got, tmp)
	}
}
