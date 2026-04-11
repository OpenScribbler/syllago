package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestCapmonDeriveCmd_Registered(t *testing.T) {
	t.Parallel()
	found := false
	for _, sub := range capmonCmd.Commands() {
		if sub.Use == "derive" {
			found = true
			break
		}
	}
	if !found {
		t.Error("derive subcommand not registered under capmonCmd")
	}
}

func TestCapmonDeriveCmd_MissingProvider(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	capmonDeriveCmd.Flags().Set("provider", "")
	defer capmonDeriveCmd.Flags().Set("provider", "")

	err := capmonDeriveCmd.RunE(capmonDeriveCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --provider is missing")
	}
}

func TestCapmonDeriveCmd_ValidFormatDoc(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	dir := t.TempDir()

	// Write canonical keys
	canonicalKeys := `content_types:
  skills:
    display_name:
      description: "Display name"
      type: string
    project_scope:
      description: "Project scope"
      type: bool
`
	canonicalKeysPath := filepath.Join(dir, "canonical-keys.yaml")
	if err := os.WriteFile(canonicalKeysPath, []byte(canonicalKeys), 0644); err != nil {
		t.Fatal(err)
	}

	// Write valid format doc
	formatsDir := filepath.Join(dir, "formats")
	if err := os.MkdirAll(formatsDir, 0755); err != nil {
		t.Fatal(err)
	}
	formatDoc := `provider: test-provider
last_fetched_at: "2026-04-11T00:00:00Z"
generation_method: human-edited
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: "2026-04-11T00:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: confirmed
`
	if err := os.WriteFile(filepath.Join(formatsDir, "test-provider.yaml"), []byte(formatDoc), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(dir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Set overrides
	origFormats := capmonDeriveFomatsDirOverride
	origOutput := capmonDeriveOutputDirOverride
	origCanonical := capmonDeriveCanonicalKeyOverride
	capmonDeriveFomatsDirOverride = formatsDir
	capmonDeriveOutputDirOverride = outputDir
	capmonDeriveCanonicalKeyOverride = canonicalKeysPath
	t.Cleanup(func() {
		capmonDeriveFomatsDirOverride = origFormats
		capmonDeriveOutputDirOverride = origOutput
		capmonDeriveCanonicalKeyOverride = origCanonical
	})

	capmonDeriveCmd.Flags().Set("provider", "test-provider")
	defer capmonDeriveCmd.Flags().Set("provider", "")

	err := capmonDeriveCmd.RunE(capmonDeriveCmd, []string{})
	if err != nil {
		t.Fatalf("expected valid format doc to derive successfully, got: %v", err)
	}

	// Verify the output file was written.
	outPath := filepath.Join(outputDir, "test-provider-skills.yaml")
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Errorf("expected derived spec at %s, file not found", outPath)
	}
}
