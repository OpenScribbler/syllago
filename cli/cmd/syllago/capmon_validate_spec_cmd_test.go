package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validApprovedSpec = `provider: test-provider
content_type: skills
format: yaml-frontmatter
proposed_mappings:
  - canonical_key: display_name
    supported: true
    mechanism: "yaml frontmatter key: name"
    confidence: confirmed
human_action: approve
reviewed_at: "2026-04-10T12:00:00Z"
`

const unapprovedSpec = `provider: test-provider
content_type: skills
format: yaml-frontmatter
proposed_mappings: []
human_action: ""
`

func TestCapmonValidateSpec_ValidApproved(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "test-provider-skills.yaml")
	if err := os.WriteFile(specPath, []byte(validApprovedSpec), 0644); err != nil {
		t.Fatal(err)
	}

	orig := capmonSpecsDirOverride
	capmonSpecsDirOverride = dir
	t.Cleanup(func() { capmonSpecsDirOverride = orig })

	capmonValidateSpecCmd.Flags().Set("provider", "test-provider")
	defer capmonValidateSpecCmd.Flags().Set("provider", "")

	err := capmonValidateSpecCmd.RunE(capmonValidateSpecCmd, []string{})
	if err != nil {
		t.Errorf("valid approved spec: expected no error, got: %v", err)
	}
}

func TestCapmonValidateSpec_UnapprovedSpec(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "test-provider-skills.yaml")
	if err := os.WriteFile(specPath, []byte(unapprovedSpec), 0644); err != nil {
		t.Fatal(err)
	}

	orig := capmonSpecsDirOverride
	capmonSpecsDirOverride = dir
	t.Cleanup(func() { capmonSpecsDirOverride = orig })

	capmonValidateSpecCmd.Flags().Set("provider", "test-provider")
	defer capmonValidateSpecCmd.Flags().Set("provider", "")

	err := capmonValidateSpecCmd.RunE(capmonValidateSpecCmd, []string{})
	if err == nil {
		t.Fatal("expected error for unapproved spec (human_action: \"\"), got nil")
	}
	if !strings.Contains(err.Error(), "test-provider") {
		t.Errorf("error should mention provider %q, got: %v", "test-provider", err)
	}
}

func TestCapmonValidateSpec_MissingProvider(t *testing.T) {
	err := capmonValidateSpecCmd.RunE(capmonValidateSpecCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --provider is not set, got nil")
	}
}

func TestCapmonValidateSpec_Registered(t *testing.T) {
	found := false
	for _, cmd := range capmonCmd.Commands() {
		if cmd.Use == "validate-spec" {
			found = true
		}
	}
	if !found {
		t.Error("validate-spec subcommand not registered under capmon")
	}
}
