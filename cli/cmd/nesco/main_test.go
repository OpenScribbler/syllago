package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/output"
)

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
	if !strings.Contains(out, "scan") || !strings.Contains(out, "version") {
		t.Error("help output should list subcommands")
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
