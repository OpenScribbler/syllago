package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestCapmonValidateSourcesCmd_Registered(t *testing.T) {
	t.Parallel()
	found := false
	for _, sub := range capmonCmd.Commands() {
		if sub.Use == "validate-sources" {
			found = true
			break
		}
	}
	if !found {
		t.Error("validate-sources subcommand not registered under capmonCmd")
	}
}

func TestCapmonValidateSourcesCmd_MissingProvider(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	capmonValidateSourcesCmd.Flags().Set("provider", "")
	defer capmonValidateSourcesCmd.Flags().Set("provider", "")

	err := capmonValidateSourcesCmd.RunE(capmonValidateSourcesCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --provider is missing")
	}
}

func TestCapmonValidateSourcesCmd_ValidManifest(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	dir := t.TempDir()
	content := `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  skills:
    sources:
      - url: "https://example.com/skills"
        type: documentation
        format: md
        selector: {}
`
	if err := os.WriteFile(filepath.Join(dir, "test-provider.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	orig := capmonSourcesDirOverride
	capmonSourcesDirOverride = dir
	t.Cleanup(func() { capmonSourcesDirOverride = orig })

	capmonValidateSourcesCmd.Flags().Set("provider", "test-provider")
	defer capmonValidateSourcesCmd.Flags().Set("provider", "")

	err := capmonValidateSourcesCmd.RunE(capmonValidateSourcesCmd, []string{})
	if err != nil {
		t.Errorf("expected valid manifest to pass, got: %v", err)
	}
}

func TestCapmonValidateSourcesCmd_BrokenManifest(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	dir := t.TempDir()
	content := `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  skills:
    sources: []
`
	if err := os.WriteFile(filepath.Join(dir, "test-provider.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	orig := capmonSourcesDirOverride
	capmonSourcesDirOverride = dir
	t.Cleanup(func() { capmonSourcesDirOverride = orig })

	capmonValidateSourcesCmd.Flags().Set("provider", "test-provider")
	defer capmonValidateSourcesCmd.Flags().Set("provider", "")

	err := capmonValidateSourcesCmd.RunE(capmonValidateSourcesCmd, []string{})
	if err == nil {
		t.Fatal("expected error for manifest with missing URIs")
	}
}
