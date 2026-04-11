package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestCapmonValidateFormatDocCmd_Registered(t *testing.T) {
	t.Parallel()
	found := false
	for _, sub := range capmonCmd.Commands() {
		if sub.Use == "validate-format-doc" {
			found = true
			break
		}
	}
	if !found {
		t.Error("validate-format-doc subcommand not registered under capmonCmd")
	}
}

func TestCapmonValidateFormatDocCmd_MissingProvider(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	capmonValidateFormatDocCmd.Flags().Set("provider", "")
	defer capmonValidateFormatDocCmd.Flags().Set("provider", "")

	err := capmonValidateFormatDocCmd.RunE(capmonValidateFormatDocCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --provider is missing")
	}
}

func TestCapmonValidateFormatDocCmd_ValidFile(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	dir := t.TempDir()

	// Write canonical keys
	canonicalKeys := `content_types:
  skills:
    display_name:
      description: "Display name"
      type: string
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
	validDoc := `provider: test-provider
last_fetched_at: "2026-04-11T00:00:00Z"
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
	if err := os.WriteFile(filepath.Join(formatsDir, "test-provider.yaml"), []byte(validDoc), 0644); err != nil {
		t.Fatal(err)
	}

	// Override dirs for test
	origFormats := capmonFormatDocsDirOverride
	origCanonical := capmonCanonicalKeysDirOverride
	capmonFormatDocsDirOverride = formatsDir
	capmonCanonicalKeysDirOverride = canonicalKeysPath
	t.Cleanup(func() {
		capmonFormatDocsDirOverride = origFormats
		capmonCanonicalKeysDirOverride = origCanonical
	})

	capmonValidateFormatDocCmd.Flags().Set("provider", "test-provider")
	defer capmonValidateFormatDocCmd.Flags().Set("provider", "")

	err := capmonValidateFormatDocCmd.RunE(capmonValidateFormatDocCmd, []string{})
	if err != nil {
		t.Errorf("expected valid format doc to pass, got: %v", err)
	}
}

func TestCapmonValidateFormatDocCmd_UnknownKey(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	dir := t.TempDir()

	canonicalKeys := `content_types:
  skills:
    display_name:
      description: "Display name"
      type: string
`
	canonicalKeysPath := filepath.Join(dir, "canonical-keys.yaml")
	if err := os.WriteFile(canonicalKeysPath, []byte(canonicalKeys), 0644); err != nil {
		t.Fatal(err)
	}

	formatsDir := filepath.Join(dir, "formats")
	if err := os.MkdirAll(formatsDir, 0755); err != nil {
		t.Fatal(err)
	}
	invalidDoc := `provider: test-provider
last_fetched_at: "2026-04-11T00:00:00Z"
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
      not_a_real_key:
        supported: true
        mechanism: "bad"
        confidence: confirmed
`
	if err := os.WriteFile(filepath.Join(formatsDir, "test-provider.yaml"), []byte(invalidDoc), 0644); err != nil {
		t.Fatal(err)
	}

	origFormats := capmonFormatDocsDirOverride
	origCanonical := capmonCanonicalKeysDirOverride
	capmonFormatDocsDirOverride = formatsDir
	capmonCanonicalKeysDirOverride = canonicalKeysPath
	t.Cleanup(func() {
		capmonFormatDocsDirOverride = origFormats
		capmonCanonicalKeysDirOverride = origCanonical
	})

	capmonValidateFormatDocCmd.Flags().Set("provider", "test-provider")
	defer capmonValidateFormatDocCmd.Flags().Set("provider", "")

	err := capmonValidateFormatDocCmd.RunE(capmonValidateFormatDocCmd, []string{})
	if err == nil {
		t.Fatal("expected error for unknown canonical key")
	}
}
