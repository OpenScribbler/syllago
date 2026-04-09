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
