package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
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
	for _, key := range []string{"version", "contentTypes", "providers", "library", "config", "registries", "commands"} {
		if _, ok := manifest[key]; !ok {
			t.Errorf("manifest missing %q key", key)
		}
	}
	// Providers should be objects with name/slug/detected fields.
	provs, ok := manifest["providers"].([]any)
	if !ok || len(provs) == 0 {
		t.Fatal("providers should be a non-empty array")
	}
	first, ok := provs[0].(map[string]any)
	if !ok {
		t.Fatal("provider entry should be an object")
	}
	if _, ok := first["slug"]; !ok {
		t.Error("provider entry missing 'slug' field")
	}
	if _, ok := first["detected"]; !ok {
		t.Error("provider entry missing 'detected' field")
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

func TestInfoFormatsShowsProviders(t *testing.T) {
	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	output.JSON = false
	defer func() { output.Writer = origWriter }()

	err := infoFormatsCmd.RunE(infoFormatsCmd, []string{})
	if err != nil {
		t.Fatalf("info formats failed: %v", err)
	}

	out := buf.String()

	// Should show which providers use each format
	if !strings.Contains(out, "claude-code") {
		t.Error("plain text output should list providers for each format")
	}

	// Should connect format to providers on the same line
	lines := strings.Split(out, "\n")
	foundFormatWithProviders := false
	for _, line := range lines {
		if (strings.Contains(line, "Markdown") || strings.Contains(line, "JSON")) &&
			strings.Contains(line, "claude-code") {
			foundFormatWithProviders = true
			break
		}
	}
	if !foundFormatWithProviders {
		t.Error("expected format lines to show provider names")
	}
}

func TestInfoTextShowsAllSections(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	err := infoCmd.RunE(infoCmd, []string{})
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}

	out := stdout.String()
	for _, section := range []string{"Library:", "Providers:", "Content types:", "Config:"} {
		if !strings.Contains(out, section) {
			t.Errorf("expected %q section in text output, got:\n%s", section, out)
		}
	}
	// Should show detection status markers
	if !strings.Contains(out, "[+]") && !strings.Contains(out, "[x]") {
		t.Errorf("expected detection status markers [+] or [x], got:\n%s", out)
	}
}

func TestInfoProvidersSlug_TextOutput(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"quality-test.yaml": extensionWithMixedQualityYAML,
	})
	origDir := infoProviderFormatsDir
	infoProviderFormatsDir = dir
	t.Cleanup(func() { infoProviderFormatsDir = origDir })

	stdout, _ := output.SetForTest(t)
	err := infoProvidersCmd.RunE(infoProvidersCmd, []string{"quality-test"})
	if err != nil {
		t.Fatalf("info providers quality-test failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "quality-test") {
		t.Errorf("output missing slug name, got:\n%s", out)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("output missing count of 1 for unspecified fields, got:\n%s", out)
	}
}

func TestInfoProvidersSlug_JSONOutput(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"quality-test.yaml": extensionWithMixedQualityYAML,
	})
	origDir := infoProviderFormatsDir
	infoProviderFormatsDir = dir
	t.Cleanup(func() { infoProviderFormatsDir = origDir })

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	err := infoProvidersCmd.RunE(infoProvidersCmd, []string{"quality-test"})
	if err != nil {
		t.Fatalf("info providers quality-test --json failed: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, stdout.String())
	}
	if result["slug"] != "quality-test" {
		t.Errorf("slug = %v, want quality-test", result["slug"])
	}
	if result["unspecified_required_count"].(float64) != 1 {
		t.Errorf("unspecified_required_count = %v, want 1", result["unspecified_required_count"])
	}
}

func TestInfoProvidersSlug_UnknownSlugError(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"quality-test.yaml": extensionWithMixedQualityYAML,
	})
	origDir := infoProviderFormatsDir
	infoProviderFormatsDir = dir
	t.Cleanup(func() { infoProviderFormatsDir = origDir })

	_, _ = output.SetForTest(t)
	err := infoProvidersCmd.RunE(infoProvidersCmd, []string{"nonexistent-provider"})
	if err == nil {
		t.Fatal("expected error for unknown provider slug")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
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
