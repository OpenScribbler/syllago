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

func TestSeedProviderCapabilities_AppliesDotPaths(t *testing.T) {
	capsDir := t.TempDir()
	seedOpts := capmon.SeedOptions{
		CapsDir:  capsDir,
		Provider: "test-provider",
		Extracted: map[string]string{
			"skills.supported": "true",
			"skills.capabilities.frontmatter_name.supported": "true",
			"skills.capabilities.frontmatter_name.mechanism": "yaml key: name",
			"hooks.events.before_tool_execute.native_name":   "PreToolUse",
			"hooks.events.before_tool_execute.blocking":      "prevent",
		},
	}
	if err := capmon.SeedProviderCapabilities(seedOpts); err != nil {
		t.Fatalf("seed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(capsDir, "test-provider.yaml"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	out := string(data)
	checks := []struct {
		want string
		desc string
	}{
		{"skills:", "skills content type entry"},
		{"supported: true", "skills supported flag"},
		{"frontmatter_name:", "frontmatter_name capability"},
		{"yaml key: name", "frontmatter_name mechanism"},
		{"before_tool_execute:", "hook event entry"},
		{"native_name: PreToolUse", "hook native_name"},
		{"blocking: prevent", "hook blocking"},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("output missing %s: %q\nFull output:\n%s", c.desc, c.want, out)
		}
	}
}
