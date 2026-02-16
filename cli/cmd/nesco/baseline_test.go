package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/drift"
	"github.com/holdenhewett/romanesco/cli/internal/output"
)

func TestBaselineCommandCreatesBaseline(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	// Redirect output
	var buf bytes.Buffer
	output.Writer = &buf
	defer func() { output.Writer = os.Stdout }()

	cmd := baselineCmd
	cmd.SetArgs([]string{})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("baseline command failed: %v", err)
	}

	nescoDir := filepath.Join(tmp, ".nesco")
	if !drift.BaselineExists(nescoDir) {
		t.Error("baseline command should create a baseline file")
	}
}

func TestBaselineCommandDoesNotEmitContextFiles(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	// Redirect output
	var buf bytes.Buffer
	output.Writer = &buf
	defer func() { output.Writer = os.Stdout }()

	cmd := baselineCmd
	cmd.SetArgs([]string{})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("baseline command failed: %v", err)
	}

	// Baseline should NOT emit context files (that's scan's job)
	if _, err := os.Stat(filepath.Join(tmp, "CLAUDE.md")); err == nil {
		t.Error("baseline command should not emit CLAUDE.md — that's scan's job")
	}
}

func TestBaselineCommandJSON(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	var buf bytes.Buffer
	output.Writer = &buf
	output.JSON = true
	defer func() {
		output.Writer = os.Stdout
		output.JSON = false
	}()

	cmd := baselineCmd
	cmd.SetArgs([]string{"--json"})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("baseline --json failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output not valid JSON: %v\nGot: %s", err, buf.String())
	}
	if _, ok := result["sections"]; !ok {
		t.Error("JSON output should include 'sections' field")
	}
	if _, ok := result["path"]; !ok {
		t.Error("JSON output should include 'path' field")
	}
}
