package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
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

func TestConfigAddUnknownProviderWarning(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stderr bytes.Buffer
	origErr := output.ErrWriter
	output.ErrWriter = &stderr
	defer func() { output.ErrWriter = origErr }()

	// Redirect stdout too to avoid noise
	var stdout bytes.Buffer
	origWriter := output.Writer
	output.Writer = &stdout
	defer func() { output.Writer = origWriter }()

	err := configAddCmd.RunE(configAddCmd, []string{"xyz123"})
	if err != nil {
		t.Fatalf("config add should succeed even with unknown slug: %v", err)
	}

	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "Warning") || !strings.Contains(stderrStr, "unknown") {
		t.Errorf("expected warning about unknown provider slug, got stderr: %q", stderrStr)
	}
}

func TestConfigAddKnownProviderNoWarning(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stderr bytes.Buffer
	origErr := output.ErrWriter
	output.ErrWriter = &stderr
	defer func() { output.ErrWriter = origErr }()

	var stdout bytes.Buffer
	origWriter := output.Writer
	output.Writer = &stdout
	defer func() { output.Writer = origWriter }()

	err := configAddCmd.RunE(configAddCmd, []string{"cursor"})
	if err != nil {
		t.Fatalf("config add failed: %v", err)
	}

	stderrStr := stderr.String()
	if strings.Contains(stderrStr, "Warning") {
		t.Errorf("should not warn for known provider, got: %s", stderrStr)
	}
}

func TestConfigAddWarningListsKnownProviders(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stderr bytes.Buffer
	origErr := output.ErrWriter
	output.ErrWriter = &stderr
	defer func() { output.ErrWriter = origErr }()

	var stdout bytes.Buffer
	origWriter := output.Writer
	output.Writer = &stdout
	defer func() { output.Writer = origWriter }()

	err := configAddCmd.RunE(configAddCmd, []string{"cursro"})
	if err != nil {
		t.Fatalf("config add should succeed: %v", err)
	}

	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "cursor") || !strings.Contains(stderrStr, "claude-code") {
		t.Errorf("warning should list known provider slugs, got: %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "Known providers") {
		t.Error("warning should have 'Known providers' label")
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

func TestConfigList_EmptyPrompt(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{})

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	// configListCmd prints via fmt.Println (os.Stdout), not output.Writer.
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	if err := configListCmd.RunE(configListCmd, nil); err != nil {
		t.Fatalf("config list: %v", err)
	}
	w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "No providers configured") {
		t.Errorf("expected empty prompt, got: %s", out)
	}
}

func TestConfigList_PopulatedProviders(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code", "cursor"}})

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	if err := configListCmd.RunE(configListCmd, nil); err != nil {
		t.Fatalf("config list: %v", err)
	}
	w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "claude-code") || !strings.Contains(out, "cursor") {
		t.Errorf("expected provider listing, got: %s", out)
	}
}

func TestConfigList_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	t.Cleanup(func() { output.JSON = false })

	if err := configListCmd.RunE(configListCmd, nil); err != nil {
		t.Fatalf("config list --json: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "claude-code") || !strings.Contains(out, `"providers"`) {
		t.Errorf("expected JSON with providers field, got: %s", out)
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
