package main

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestCapmonVerify_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	orig := capmonCapabilitiesDirOverride
	capmonCapabilitiesDirOverride = dir
	t.Cleanup(func() { capmonCapabilitiesDirOverride = orig })

	err := capmonVerifyCmd.RunE(capmonVerifyCmd, []string{})
	if err != nil {
		t.Errorf("verify on empty dir: %v", err)
	}
}

func TestCapmonVerify_StalenessCheck_ManifestMissing(t *testing.T) {
	// When --staleness-check is set and no last-run.json exists, an issue should be opened.
	cacheDir := t.TempDir()
	issueCreated := false
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "issue" {
				issueCreated = true
			}
		}
		return []byte("https://github.com/test/repo/issues/99"), nil
	})
	t.Cleanup(func() { capmon.SetGHCommandForTest(nil) })

	capmonVerifyCmd.Flags().Set("staleness-check", "true")
	capmonVerifyCmd.Flags().Set("threshold-hours", "36")
	capmonVerifyCmd.Flags().Set("cache-root", cacheDir)
	defer func() {
		capmonVerifyCmd.Flags().Set("staleness-check", "false")
		capmonVerifyCmd.Flags().Set("cache-root", "")
	}()

	err := capmonVerifyCmd.RunE(capmonVerifyCmd, []string{})
	if err != nil {
		t.Errorf("staleness check with missing manifest: %v", err)
	}
	if !issueCreated {
		t.Error("expected GH issue to be created when manifest is missing")
	}
}

func TestCapmonCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "capmon" {
			found = true
			break
		}
	}
	if !found {
		t.Error("capmon command not registered on rootCmd")
	}
}

func TestCapmonFetch_InvalidSlug(t *testing.T) {
	capmonFetchCmd.SetArgs([]string{"--provider", "INVALID SLUG"})
	err := capmonFetchCmd.RunE(capmonFetchCmd, []string{})
	if err == nil {
		t.Error("expected error for invalid provider slug")
	}
}

func TestCapmonExtract_Registered(t *testing.T) {
	found := false
	for _, cmd := range capmonCmd.Commands() {
		if cmd.Use == "extract" {
			found = true
		}
	}
	if !found {
		t.Error("extract subcommand not registered under capmon")
	}
}

func TestCapmonRun_StageFlag(t *testing.T) {
	// Valid stage values should not produce a flag-parse error
	validStages := []string{"fetch-extract", "report", ""}
	for _, stage := range validStages {
		args := []string{}
		if stage != "" {
			args = append(args, "--stage", stage)
		}
		// Just check the flag parses — actual execution will fail on missing dirs
		capmonRunCmd.ParseFlags(args)
		got, _ := capmonRunCmd.Flags().GetString("stage")
		if stage != "" && got != stage {
			t.Errorf("stage flag: got %q, want %q", got, stage)
		}
	}
}

func TestCapmonTestFixtures_Registered(t *testing.T) {
	found := false
	for _, cmd := range capmonCmd.Commands() {
		if cmd.Use == "test-fixtures" {
			found = true
		}
	}
	if !found {
		t.Error("test-fixtures subcommand not registered under capmon")
	}
}

func TestCapmonTestFixtures_RefusesWithoutProvider(t *testing.T) {
	capmonTestFixturesCmd.Flags().Set("update", "true")
	defer capmonTestFixturesCmd.Flags().Set("update", "false")
	err := capmonTestFixturesCmd.RunE(capmonTestFixturesCmd, []string{})
	if err == nil {
		t.Error("expected error: --update requires --provider")
	}
}
