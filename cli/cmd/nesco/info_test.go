package main

import (
	"bytes"
	"encoding/json"
	"strings"
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

func TestInfoProvidersUsesDisplayNames(t *testing.T) {
	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	output.JSON = true
	defer func() {
		output.JSON = false
		output.Writer = origWriter
	}()

	err := infoProvidersCmd.RunE(infoProvidersCmd, []string{})
	if err != nil {
		t.Fatalf("info providers failed: %v", err)
	}

	var infos []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &infos); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Find a provider with supported types and verify they use Label() (title case)
	for _, info := range infos {
		types, ok := info["supportedTypes"].([]any)
		if !ok || len(types) == 0 {
			continue
		}
		for _, typ := range types {
			s, ok := typ.(string)
			if !ok {
				continue
			}
			// Labels should be title case (e.g., "Skills" not "skills")
			if s != "" && strings.ToLower(s) == s {
				t.Errorf("supportedTypes should use display names (title case), got: %q", s)
			}
		}
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
