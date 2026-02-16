package main

import (
	"bytes"
	"strings"
	"testing"
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
