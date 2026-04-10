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

func TestSeedProviderCapabilities_WritesConfidence(t *testing.T) {
	capsDir := t.TempDir()
	seedOpts := capmon.SeedOptions{
		CapsDir:  capsDir,
		Provider: "test-provider",
		Extracted: map[string]string{
			"skills.supported":                            "true",
			"skills.capabilities.display_name.supported":  "true",
			"skills.capabilities.display_name.mechanism":  "yaml frontmatter key: name",
			"skills.capabilities.display_name.confidence": "confirmed",
			"skills.capabilities.description.supported":   "true",
			"skills.capabilities.description.mechanism":   "yaml frontmatter key: description",
			"skills.capabilities.description.confidence":  "inferred",
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
	if !strings.Contains(out, "confidence: confirmed") {
		t.Errorf("missing 'confidence: confirmed' in output:\n%s", out)
	}
	if !strings.Contains(out, "confidence: inferred") {
		t.Errorf("missing 'confidence: inferred' in output:\n%s", out)
	}
}

func TestSeedProviderCapabilities_UnapprovedSpec(t *testing.T) {
	capsDir := t.TempDir()
	specsDir := t.TempDir()

	// Write a spec with no human_action (unapproved)
	specContent := `provider: test-provider
content_type: skills
format: markdown
proposed_mappings: []
human_action: ""
`
	specPath := filepath.Join(specsDir, "test-provider-skills.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	opts := capmon.SeedOptions{
		CapsDir:        capsDir,
		Provider:       "test-provider",
		SeederSpecsDir: specsDir,
		Extracted: map[string]string{
			"skills.supported": "true",
		},
	}
	err := capmon.SeedProviderCapabilities(opts)
	if err == nil {
		t.Fatal("expected error for unapproved spec, got nil")
	}
	if !strings.Contains(err.Error(), "human_action") {
		t.Errorf("error %q should mention human_action", err.Error())
	}
}

func TestSeedProviderCapabilities_NoSpecFile_SkipsGate(t *testing.T) {
	// When SeederSpecsDir is empty string, the gate is skipped entirely
	capsDir := t.TempDir()
	opts := capmon.SeedOptions{
		CapsDir:        capsDir,
		Provider:       "test-provider",
		SeederSpecsDir: "", // gate disabled
		Extracted: map[string]string{
			"skills.supported": "true",
		},
	}
	if err := capmon.SeedProviderCapabilities(opts); err != nil {
		t.Fatalf("expected no error with no SeederSpecsDir, got: %v", err)
	}
}

func TestSeedProviderCapabilities_ApprovedSpec_Proceeds(t *testing.T) {
	capsDir := t.TempDir()
	specsDir := t.TempDir()

	specContent := `provider: test-provider
content_type: skills
format: markdown
proposed_mappings: []
human_action: approve
reviewed_at: "2026-04-10T00:00:00Z"
`
	specPath := filepath.Join(specsDir, "test-provider-skills.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	opts := capmon.SeedOptions{
		CapsDir:        capsDir,
		Provider:       "test-provider",
		SeederSpecsDir: specsDir,
		Extracted: map[string]string{
			"skills.supported": "true",
		},
	}
	if err := capmon.SeedProviderCapabilities(opts); err != nil {
		t.Fatalf("unexpected error with approved spec: %v", err)
	}
}

func TestSeedCrushAndRooCodeE2E(t *testing.T) {
	// This test exercises the full pipeline:
	// cache extraction → recognition → capability YAML writing
	// It uses the real .capmon-cache directory relative to the project root.
	cacheRoot := filepath.Join("..", "..", "..", ".capmon-cache")
	if _, err := os.Stat(cacheRoot); os.IsNotExist(err) {
		t.Skip("no .capmon-cache directory — run capmon fetch first")
	}

	for _, provider := range []string{"crush", "roo-code"} {
		provider := provider
		t.Run(provider, func(t *testing.T) {
			dotPaths, err := capmon.LoadAndRecognizeCache(cacheRoot, provider)
			if err != nil {
				t.Fatalf("LoadAndRecognizeCache(%q): %v", provider, err)
			}
			if len(dotPaths) == 0 {
				t.Fatalf("LoadAndRecognizeCache(%q): returned empty map — real recognizer should produce output", provider)
			}
			// Verify key canonical fields are present
			if dotPaths["skills.supported"] != "true" {
				t.Errorf("%s: skills.supported missing or not 'true'", provider)
			}
			if dotPaths["skills.capabilities.display_name.confidence"] != "confirmed" {
				t.Errorf("%s: display_name.confidence not 'confirmed', got %q", provider, dotPaths["skills.capabilities.display_name.confidence"])
			}
			if dotPaths["skills.capabilities.project_scope.supported"] != "true" {
				t.Errorf("%s: project_scope.supported missing", provider)
			}
			if dotPaths["skills.capabilities.canonical_filename.supported"] != "true" {
				t.Errorf("%s: canonical_filename.supported missing", provider)
			}

			// Write to temp output dir — verify YAML is produced with confidence fields
			capsDir := t.TempDir()
			opts := capmon.SeedOptions{
				CapsDir:        capsDir,
				Provider:       provider,
				Extracted:      dotPaths,
				SeederSpecsDir: "", // skip gate in this test
			}
			if err := capmon.SeedProviderCapabilities(opts); err != nil {
				t.Fatalf("SeedProviderCapabilities(%q): %v", provider, err)
			}
			data, err := os.ReadFile(filepath.Join(capsDir, provider+".yaml"))
			if err != nil {
				t.Fatalf("read output YAML: %v", err)
			}
			out := string(data)
			if !strings.Contains(out, "confidence: confirmed") {
				t.Errorf("%s: output YAML missing 'confidence: confirmed'\n%s", provider, out)
			}
			if !strings.Contains(out, "canonical_filename") {
				t.Errorf("%s: output YAML missing 'canonical_filename'\n%s", provider, out)
			}
		})
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
