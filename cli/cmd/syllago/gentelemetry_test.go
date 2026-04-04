package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

// TestGentelemetry verifies the top-level manifest structure and required fields.
func TestGentelemetry(t *testing.T) {
	raw := captureStdout(t, func() {
		if err := gentelemetryCmd.RunE(gentelemetryCmd, nil); err != nil {
			t.Fatalf("_gentelemetry failed: %v", err)
		}
	})

	var manifest TelemetryManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("output is not valid JSON: %v\nfirst 200 bytes: %s", err, string(raw[:min(200, len(raw))]))
	}

	if manifest.Version != "1" {
		t.Errorf("version = %q, want %q", manifest.Version, "1")
	}
	if manifest.SyllagoVersion == "" {
		t.Error("syllagoVersion is empty")
	}
	if manifest.GeneratedAt == "" {
		t.Error("generatedAt is empty")
	}
	if len(manifest.Events) == 0 {
		t.Error("events is empty")
	}
	if len(manifest.StandardProperties) == 0 {
		t.Error("standardProperties is empty")
	}
	if len(manifest.NeverCollected) == 0 {
		t.Error("neverCollected is empty")
	}
}

// TestGentelemetry_EventsComplete verifies every event has name, description,
// firedWhen, and at least one property.
func TestGentelemetry_EventsComplete(t *testing.T) {
	for _, ev := range telemetry.EventCatalog() {
		t.Run(ev.Name, func(t *testing.T) {
			if ev.Name == "" {
				t.Error("event has empty name")
			}
			if ev.Description == "" {
				t.Errorf("event %q has empty description", ev.Name)
			}
			if ev.FiredWhen == "" {
				t.Errorf("event %q has empty firedWhen", ev.Name)
			}
			if len(ev.Properties) == 0 {
				t.Errorf("event %q has no properties", ev.Name)
			}
		})
	}
}

// TestGentelemetry_PropertiesComplete verifies every property on every event has
// name, a valid type, description, a non-nil example, and at least one command.
func TestGentelemetry_PropertiesComplete(t *testing.T) {
	validTypes := map[string]bool{"string": true, "int": true, "bool": true}

	for _, ev := range telemetry.EventCatalog() {
		for _, prop := range ev.Properties {
			t.Run(ev.Name+"/"+prop.Name, func(t *testing.T) {
				if prop.Name == "" {
					t.Error("property has empty name")
				}
				if !validTypes[prop.Type] {
					t.Errorf("property %q type = %q, want one of string/int/bool", prop.Name, prop.Type)
				}
				if prop.Description == "" {
					t.Errorf("property %q has empty description", prop.Name)
				}
				if prop.Example == nil {
					t.Errorf("property %q has nil example", prop.Name)
				}
				if len(prop.Commands) == 0 {
					t.Errorf("property %q has no commands", prop.Name)
				}
			})
		}
	}
}

// TestGentelemetry_StandardProperties verifies version, os, and arch are present.
func TestGentelemetry_StandardProperties(t *testing.T) {
	props := telemetry.StandardProperties()
	propByName := make(map[string]telemetry.PropertyDef, len(props))
	for _, p := range props {
		propByName[p.Name] = p
	}

	for _, want := range []string{"version", "os", "arch"} {
		p, ok := propByName[want]
		if !ok {
			t.Errorf("standardProperties missing %q", want)
			continue
		}
		if p.Type != "string" {
			t.Errorf("standardProperties[%q].type = %q, want %q", want, p.Type, "string")
		}
		if p.Description == "" {
			t.Errorf("standardProperties[%q].description is empty", want)
		}
	}
}

// TestGentelemetry_PrivacyGuarantees verifies at least 6 entries covering the
// key categories documented in the design: file contents, paths, identity,
// registry URLs, content names, and interaction details.
func TestGentelemetry_PrivacyGuarantees(t *testing.T) {
	entries := telemetry.NeverCollected()
	if len(entries) < 6 {
		t.Errorf("neverCollected has %d entries, want >= 6", len(entries))
	}

	// Build a searchable corpus of all categories (lowercase).
	corpus := make([]string, len(entries))
	for i, e := range entries {
		if e.Category == "" {
			t.Errorf("neverCollected[%d] has empty category", i)
		}
		if e.Examples == "" {
			t.Errorf("neverCollected[%d] (%q) has empty examples", i, e.Category)
		}
		corpus[i] = strings.ToLower(e.Category)
	}

	wantCategories := []struct {
		keyword string
		label   string
	}{
		{"file", "file contents or paths"},
		{"path", "file paths"},
		{"identity", "user identity"},
		{"registry", "registry URLs"},
		{"content", "content names"},
		{"interaction", "interaction details"},
	}

	for _, wc := range wantCategories {
		found := false
		for _, c := range corpus {
			if strings.Contains(c, wc.keyword) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("neverCollected missing a %q category (keyword: %q)", wc.label, wc.keyword)
		}
	}
}

// TestGentelemetry_CatalogMatchesEnrichCalls scans all *.go files in
// cli/cmd/syllago/ for telemetry.Enrich("key", ...) calls and verifies that
// every discovered key exists as a property on at least one event in EventCatalog().
// This is a strict drift-detection test — CI fails on any mismatch.
func TestGentelemetry_CatalogMatchesEnrichCalls(t *testing.T) {
	// Find the repo root relative to this test file's location.
	// The test binary runs from cli/cmd/syllago/, so we walk upward.
	cmdDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("cannot determine test directory: %v", err)
	}
	// If the test directory doesn't look right (e.g. during go test ./...),
	// derive it from the source file path.
	if !strings.HasSuffix(cmdDir, filepath.Join("cmd", "syllago")) {
		// Attempt to find it relative to a known anchor.
		repoRoot := findRepoRoot(t)
		cmdDir = filepath.Join(repoRoot, "cli", "cmd", "syllago")
	}

	// Regex matches: telemetry.Enrich("someKey"
	enrichRe := regexp.MustCompile(`telemetry\.Enrich\("([^"]+)"`)

	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("cannot read cmd/syllago: %v", err)
	}

	seenKeys := make(map[string][]string) // key → []file
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		// Skip test files — they don't fire real events.
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cmdDir, entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		for _, match := range enrichRe.FindAllSubmatch(data, -1) {
			key := string(match[1])
			seenKeys[key] = append(seenKeys[key], entry.Name())
		}
	}

	if len(seenKeys) == 0 {
		t.Fatal("no telemetry.Enrich() calls found — scan may be broken")
	}

	// Build the set of property names across all events.
	catalogKeys := make(map[string]bool)
	for _, ev := range telemetry.EventCatalog() {
		for _, prop := range ev.Properties {
			catalogKeys[prop.Name] = true
		}
	}

	// Every key found in source must appear in the catalog.
	for key, files := range seenKeys {
		if !catalogKeys[key] {
			t.Errorf("telemetry.Enrich(%q) found in %v but %q is not in EventCatalog()", key, files, key)
		}
	}
}

// findRepoRoot walks upward from the working directory to find the repo root
// by looking for the .git directory.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (.git directory)")
		}
		dir = parent
	}
}
