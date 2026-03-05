package main

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

func TestPublishCmdRegisters(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "publish <name>" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected publish command registered on rootCmd")
	}
}

func TestPublishCmdFlagsRegistered(t *testing.T) {
	registryFlag := publishCmd.Flags().Lookup("registry")
	if registryFlag == nil {
		t.Error("expected --registry flag on publish command")
	}

	typeFlag := publishCmd.Flags().Lookup("type")
	if typeFlag == nil {
		t.Error("expected --type flag on publish command")
	}

	noInputFlag := publishCmd.Flags().Lookup("no-input")
	if noInputFlag == nil {
		t.Error("expected --no-input flag on publish command")
	}
}

func TestPublishCmdRegistryFlagRequired(t *testing.T) {
	// Verify cobra marks --registry as required via its annotations.
	registryFlag := publishCmd.Flags().Lookup("registry")
	if registryFlag == nil {
		t.Fatal("--registry flag not found")
	}
	annotations := registryFlag.Annotations
	if _, ok := annotations[cobra.BashCompOneRequiredFlag]; !ok {
		t.Error("expected --registry to be marked as required")
	}
}

func TestPublishCmdValidatesArgs(t *testing.T) {
	publishCmd.SilenceUsage = true
	publishCmd.SilenceErrors = true

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, true},
		{"two args", []string{"first", "second"}, true},
		// One arg is correct — arg validation passes; RunE will fail later.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := publishCmd.Args(publishCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestPublishItemNotFound(t *testing.T) {
	lib := setupConvertLibrary(t) // creates a skill named "my-skill"
	withConvertLibrary(t, lib)

	_, _ = output.SetForTest(t)

	publishCmd.Flags().Set("registry", "my-registry")
	publishCmd.Flags().Set("type", "")
	defer func() {
		publishCmd.Flags().Set("registry", "")
		publishCmd.Flags().Set("type", "")
	}()

	err := publishCmd.RunE(publishCmd, []string{"nonexistent-item"})
	if err == nil {
		t.Fatal("expected error for nonexistent item")
	}
	if !strings.Contains(err.Error(), "no item named") {
		t.Errorf("expected 'no item named' in error, got: %v", err)
	}
}

func TestPublishUsesGlobalLibraryOnly(t *testing.T) {
	_, _ = output.SetForTest(t)

	// Item exists but only in a non-global source — should not be found.
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-skill", Type: catalog.Skills, Source: "registry-foo"},
		},
	}

	_, err := findLibraryItem(cat, "my-skill", "")
	if err == nil {
		t.Fatal("expected error: non-global items should not be found by publish")
	}
	if !strings.Contains(err.Error(), "no item named") {
		t.Errorf("expected 'no item named' in error, got: %v", err)
	}
}
