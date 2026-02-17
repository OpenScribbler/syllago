package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/drift"
	"github.com/holdenhewett/romanesco/cli/internal/output"
)

func TestScanCommandJSON(t *testing.T) {
	tmp := setupGoProject(t)

	var buf bytes.Buffer
	origWriter := output.Writer
	origQuiet := output.Quiet
	output.Writer = &buf
	output.JSON = true
	output.Quiet = false
	defer func() {
		output.Writer = origWriter
		output.JSON = false
		output.Quiet = origQuiet
	}()

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	err := scanCmd.RunE(scanCmd, []string{})
	if err != nil {
		t.Fatalf("scan --json failed: %v", err)
	}

	// Output should be valid JSON with sections
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nGot: %s", err, buf.String())
	}
}

func TestScanDryRunDoesNotWrite(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	// Redirect output to suppress prints
	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	defer func() { output.Writer = origWriter }()

	scanCmd.Flags().Set("dry-run", "true")
	defer scanCmd.Flags().Set("dry-run", "false")

	err := scanCmd.RunE(scanCmd, []string{})
	if err != nil {
		t.Fatalf("scan --dry-run failed: %v", err)
	}

	// Baseline should NOT be updated on dry run
	nescoDir := tmp + "/.nesco"
	if drift.BaselineExists(nescoDir) {
		t.Error("dry run should not create a baseline")
	}
}

func TestScanCreatesBaseline(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	// Redirect output to suppress prints
	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	defer func() { output.Writer = origWriter }()

	err := scanCmd.RunE(scanCmd, []string{})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	nescoDir := tmp + "/.nesco"
	if !drift.BaselineExists(nescoDir) {
		t.Error("scan should create a baseline")
	}
}

// TestScanNoProjectFails verifies exit code 2 semantics when no project is detected.
func TestScanNoProjectFails(t *testing.T) {
	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return "", os.ErrNotExist }
	defer func() { findProjectRoot = origFindRoot }()

	var buf bytes.Buffer
	origWriter := output.Writer
	origErrWriter := output.ErrWriter
	output.Writer = &buf
	output.ErrWriter = &buf
	defer func() {
		output.Writer = origWriter
		output.ErrWriter = origErrWriter
	}()

	err := scanCmd.RunE(scanCmd, []string{})
	if err == nil {
		t.Error("scan should fail when no project root is found")
	}
}

// TestScanErrorIsSilent verifies that scan returns SilentError after PrintError
// so main() doesn't double-print the error message.
func TestScanErrorIsSilent(t *testing.T) {
	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return "", os.ErrNotExist }
	defer func() { findProjectRoot = origFindRoot }()

	var buf bytes.Buffer
	origWriter := output.Writer
	origErrWriter := output.ErrWriter
	output.Writer = &buf
	output.ErrWriter = &buf
	defer func() {
		output.Writer = origWriter
		output.ErrWriter = origErrWriter
	}()

	err := scanCmd.RunE(scanCmd, []string{})
	if err == nil {
		t.Fatal("expected error from scan with no project")
	}
	if !output.IsSilentError(err) {
		t.Error("scan should return SilentError after PrintError to prevent duplicate printing")
	}
}

func TestScanConfigSaveWarning(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	// Create .nesco dir as read+execute but no write to force Save error
	nescoDir := filepath.Join(tmp, ".nesco")
	os.MkdirAll(nescoDir, 0755)
	os.Chmod(nescoDir, 0555)
	defer os.Chmod(nescoDir, 0755)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	// Set env to bypass prompt (triggers auto-detect + save path)
	origEnv := os.Getenv("NESCO_NO_PROMPT")
	os.Setenv("NESCO_NO_PROMPT", "1")
	defer os.Setenv("NESCO_NO_PROMPT", origEnv)

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	origWriter := output.Writer
	origErrWriter := output.ErrWriter
	output.Writer = &outBuf
	output.ErrWriter = &errBuf
	defer func() {
		output.Writer = origWriter
		output.ErrWriter = origErrWriter
	}()

	// Run scan — may error for other reasons, we only care about the warning
	_ = scanCmd.RunE(scanCmd, []string{})

	stderrStr := errBuf.String()
	if !strings.Contains(stderrStr, "Warning") || !strings.Contains(stderrStr, "config") {
		t.Errorf("expected warning about config save failure, got stderr: %q", stderrStr)
	}
}
