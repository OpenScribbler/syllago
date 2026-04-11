package main

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestCapmonCheckCmd_Registered(t *testing.T) {
	t.Parallel()
	found := false
	for _, sub := range capmonCmd.Commands() {
		if sub.Use == "check" {
			found = true
			break
		}
	}
	if !found {
		t.Error("check subcommand not registered under capmonCmd")
	}
}

func TestCapmonCheckCmd_MutualExclusion(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	capmonCheckCmd.Flags().Set("all", "true")
	capmonCheckCmd.Flags().Set("provider", "some-provider")
	defer func() {
		capmonCheckCmd.Flags().Set("all", "false")
		capmonCheckCmd.Flags().Set("provider", "")
	}()

	err := capmonCheckCmd.RunE(capmonCheckCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --all and --provider are both set")
	}
}

func TestCapmonCheckCmd_MissingBothFlags(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	capmonCheckCmd.Flags().Set("all", "false")
	capmonCheckCmd.Flags().Set("provider", "")
	defer func() {
		capmonCheckCmd.Flags().Set("all", "false")
		capmonCheckCmd.Flags().Set("provider", "")
	}()

	err := capmonCheckCmd.RunE(capmonCheckCmd, []string{})
	if err == nil {
		t.Fatal("expected error when neither --all nor --provider is set")
	}
}

func TestCapmonCheckCmd_DryRunFlag(t *testing.T) {
	// Just verify --dry-run flag is accepted without error (not that it runs the full pipeline).
	flag := capmonCheckCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Error("--dry-run flag not registered on capmon check command")
	}
}

func TestCapmonCheckCmd_InvalidProvider(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	capmonCheckCmd.Flags().Set("provider", "INVALID SLUG")
	capmonCheckCmd.Flags().Set("all", "false")
	defer func() {
		capmonCheckCmd.Flags().Set("provider", "")
	}()

	err := capmonCheckCmd.RunE(capmonCheckCmd, []string{})
	if err == nil {
		t.Fatal("expected error for invalid provider slug")
	}
}
