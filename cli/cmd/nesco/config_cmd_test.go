package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/config"
	"github.com/holdenhewett/romanesco/cli/internal/output"
)

func TestConfigAddAndRemove(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	if err := configAddCmd.RunE(configAddCmd, []string{"cursor"}); err != nil {
		t.Fatalf("config add: %v", err)
	}

	cfg, _ := config.Load(tmp)
	if len(cfg.Providers) != 2 {
		t.Errorf("expected 2 providers after add, got %d", len(cfg.Providers))
	}

	if err := configRemoveCmd.RunE(configRemoveCmd, []string{"cursor"}); err != nil {
		t.Fatalf("config remove: %v", err)
	}

	cfg, _ = config.Load(tmp)
	if len(cfg.Providers) != 1 {
		t.Errorf("expected 1 provider after remove, got %d", len(cfg.Providers))
	}
}

func TestConfigAddDuplicate(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	err := configAddCmd.RunE(configAddCmd, []string{"claude-code"})
	if err == nil {
		t.Error("adding duplicate provider should fail")
	}
}

func TestConfigRemoveNotFound(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	err := configRemoveCmd.RunE(configRemoveCmd, []string{"nonexistent"})
	if err == nil {
		t.Error("removing nonexistent provider should fail")
	}
}

func TestConfigAddConfirmation(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stdout bytes.Buffer
	origWriter := output.Writer
	output.Writer = &stdout
	defer func() { output.Writer = origWriter }()

	if err := configAddCmd.RunE(configAddCmd, []string{"cursor"}); err != nil {
		t.Fatalf("config add: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Added") || !strings.Contains(out, "cursor") {
		t.Errorf("expected confirmation message, got: %s", out)
	}
}

func TestConfigRemoveConfirmation(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stdout bytes.Buffer
	origWriter := output.Writer
	output.Writer = &stdout
	defer func() { output.Writer = origWriter }()

	if err := configRemoveCmd.RunE(configRemoveCmd, []string{"claude-code"}); err != nil {
		t.Fatalf("config remove: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Removed") || !strings.Contains(out, "claude-code") {
		t.Errorf("expected confirmation message, got: %s", out)
	}
}
