package main

import (
	"os"
	"path/filepath"
	"strings"
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

// TestCapmonValidateFormatDocCmd_WarningPrintedToStderr verifies that non-blocking
// allow-list violations (e.g. unknown value_type) do NOT fail the command, but
// are surfaced on stderr with the dedup key and field path so operators can see
// them and CI can file tracking issues.
func TestCapmonValidateFormatDocCmd_WarningPrintedToStderr(t *testing.T) {
	stdout, stderr := output.SetForTest(t)

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
	// Doc passes blocking validation but has a value_type outside the allow-list,
	// which should produce a non-blocking warning.
	docWithWarning := `provider: test-provider
last_fetched_at: "2026-04-15T00:00:00Z"
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: "2026-04-15T00:00:00Z"
    canonical_mappings: {}
    provider_extensions:
      - id: bad_type_ext
        name: "Bad Type Ext"
        description: "test extension with non-allowed value_type"
        source_ref: "https://example.com"
        value_type: "not-in-allow-list"
`
	if err := os.WriteFile(filepath.Join(formatsDir, "test-provider.yaml"), []byte(docWithWarning), 0644); err != nil {
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
	if err != nil {
		t.Fatalf("warnings must not fail the command, got: %v", err)
	}

	stderrOut := stderr.String()
	stdoutOut := stdout.String()

	if !strings.Contains(stderrOut, "validation warning") {
		t.Errorf("expected stderr to contain %q, got: %s", "validation warning", stderrOut)
	}
	if !strings.Contains(stderrOut, "value_type") {
		t.Errorf("expected stderr to mention the failing field %q, got: %s", "value_type", stderrOut)
	}
	if !strings.Contains(stderrOut, "not-in-allow-list") {
		t.Errorf("expected stderr to quote the offending value, got: %s", stderrOut)
	}
	// Success summary goes to stdout, not stderr.
	if !strings.Contains(stdoutOut, "Schema valid") {
		t.Errorf("expected stdout to show the success marker, got: %s", stdoutOut)
	}
	if strings.Contains(stdoutOut, "validation warning") {
		t.Errorf("warnings must go to stderr, not stdout; got stdout: %s", stdoutOut)
	}
}
