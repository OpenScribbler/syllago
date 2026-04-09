package capmon_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestSeedProviderCapabilities_Idempotent(t *testing.T) {
	capsDir := t.TempDir()
	seedOpts := capmon.SeedOptions{
		CapsDir:  capsDir,
		Provider: "test-provider",
		Extracted: map[string]string{
			"hooks.events.before_tool_execute.native_name": "PreToolUse",
		},
	}
	// First run
	if err := capmon.SeedProviderCapabilities(seedOpts); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	data1, _ := os.ReadFile(filepath.Join(capsDir, "test-provider.yaml"))
	// Second run
	if err := capmon.SeedProviderCapabilities(seedOpts); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	data2, _ := os.ReadFile(filepath.Join(capsDir, "test-provider.yaml"))
	if string(data1) != string(data2) {
		t.Error("seed is not idempotent: output changed on second run")
	}
}

func TestSeedProviderCapabilities_PreservesExclusive(t *testing.T) {
	capsDir := t.TempDir()
	// Write initial file with provider_exclusive section
	initial := `schema_version: "1"
slug: test-provider
provider_exclusive:
  events:
    - native_name: CustomEvent
      description: a custom event
`
	if err := os.WriteFile(filepath.Join(capsDir, "test-provider.yaml"), []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	seedOpts := capmon.SeedOptions{
		CapsDir:  capsDir,
		Provider: "test-provider",
		Extracted: map[string]string{
			"hooks.events.before_tool_execute.native_name": "PreToolUse",
		},
		ForceOverwriteExclusive: false,
	}
	if err := capmon.SeedProviderCapabilities(seedOpts); err != nil {
		t.Fatalf("seed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(capsDir, "test-provider.yaml"))
	if !strings.Contains(string(data), "CustomEvent") {
		t.Error("provider_exclusive entry CustomEvent was removed without --force-overwrite-exclusive")
	}
}
