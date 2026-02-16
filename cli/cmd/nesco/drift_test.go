package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/drift"
	"github.com/holdenhewett/romanesco/cli/internal/model"
	"github.com/holdenhewett/romanesco/cli/internal/output"
)

func TestDriftCommandNoBaseline(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	// Redirect error output
	var buf bytes.Buffer
	output.ErrWriter = &buf
	defer func() { output.ErrWriter = os.Stderr }()

	cmd := driftCmd
	cmd.SetArgs([]string{})

	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Error("drift without baseline should return error")
	}
}

func TestDriftCommandClean(t *testing.T) {
	tmp := setupGoProject(t)
	nescoDir := filepath.Join(tmp, ".nesco")

	// Create a baseline that matches current state (empty doc)
	doc := model.ContextDocument{ProjectName: "test"}
	drift.SaveBaseline(nescoDir, doc)

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

	cmd := driftCmd
	cmd.SetArgs([]string{"--json"})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("drift failed: %v", err)
	}

	var report drift.DriftReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
}

func TestDriftCommandJSON(t *testing.T) {
	tmp := setupGoProject(t)
	nescoDir := filepath.Join(tmp, ".nesco")

	// Create an empty baseline, scan will find things -> drift
	doc := model.ContextDocument{ProjectName: "test"}
	drift.SaveBaseline(nescoDir, doc)

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

	cmd := driftCmd
	cmd.SetArgs([]string{"--json"})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("drift --json failed: %v", err)
	}

	// Should produce valid JSON regardless of drift status
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output not valid JSON: %v\nGot: %s", err, buf.String())
	}
}
