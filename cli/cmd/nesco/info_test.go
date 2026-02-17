package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/output"
)

func TestInfoJSON(t *testing.T) {
	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	output.JSON = true
	defer func() {
		output.JSON = false
		output.Writer = origWriter
	}()

	err := infoCmd.RunE(infoCmd, []string{})
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}

	var manifest map[string]any
	if err := json.Unmarshal(buf.Bytes(), &manifest); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if _, ok := manifest["version"]; !ok {
		t.Error("manifest missing 'version' key")
	}
	if _, ok := manifest["contentTypes"]; !ok {
		t.Error("manifest missing 'contentTypes' key")
	}
	if _, ok := manifest["providers"]; !ok {
		t.Error("manifest missing 'providers' key")
	}
}

func TestInfoDevBuild(t *testing.T) {
	oldVersion := version
	version = ""
	defer func() { version = oldVersion }()

	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	output.JSON = true
	defer func() {
		output.JSON = false
		output.Writer = origWriter
	}()

	err := infoCmd.RunE(infoCmd, []string{})
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}

	var manifest map[string]any
	if err := json.Unmarshal(buf.Bytes(), &manifest); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	ver, ok := manifest["version"].(string)
	if !ok {
		t.Fatal("version field missing or not a string")
	}
	if ver != "(dev build)" {
		t.Errorf("version = %q, want %q", ver, "(dev build)")
	}
}
