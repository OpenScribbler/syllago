package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/config"
)

func TestInitCreatesConfig(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Reset flag state
	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init --yes failed: %v", err)
	}

	if !config.Exists(tmp) {
		t.Error("config.json should exist after init")
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	err := initCmd.RunE(initCmd, []string{})
	if err == nil {
		t.Error("init should fail when config already exists (no --force)")
	}
}

func TestInitForceOverwrite(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"old"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.Flags().Set("force", "true")
	initCmd.Flags().Set("yes", "true")
	err := initCmd.RunE(initCmd, []string{})
	if err != nil {
		t.Fatalf("init --force --yes failed: %v", err)
	}
}
