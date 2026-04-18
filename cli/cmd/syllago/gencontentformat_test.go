package main

import (
	"encoding/json"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/contentformat"
)

func TestGencontentformat_EmitsValidJSON(t *testing.T) {
	raw := captureStdout(t, func() {
		if err := gencontentformatCmd.RunE(gencontentformatCmd, nil); err != nil {
			t.Fatalf("_gencontentformat failed: %v", err)
		}
	})

	var manifest ContentFormatManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if manifest.Version != "1" {
		t.Errorf("version = %q, want %q", manifest.Version, "1")
	}
	if manifest.GeneratedAt == "" {
		t.Error("generated_at is empty")
	}
	if manifest.SyllagoVersion == "" {
		t.Error("syllago_version is empty")
	}
}

func TestGencontentformat_AllEnumsPopulated(t *testing.T) {
	raw := captureStdout(t, func() {
		_ = gencontentformatCmd.RunE(gencontentformatCmd, nil)
	})

	var manifest ContentFormatManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tests := []struct {
		name   string
		values []string
	}{
		{"effort", manifest.Enums.Effort},
		{"permission_mode", manifest.Enums.PermissionMode},
		{"source_type", manifest.Enums.SourceType},
		{"source_visibility", manifest.Enums.SourceVisibility},
		{"source_scope", manifest.Enums.SourceScope},
		{"content_type", manifest.Enums.ContentType},
		{"hook_handler_type", manifest.Enums.HookHandlerType},
	}

	for _, tt := range tests {
		if len(tt.values) == 0 {
			t.Errorf("%s: empty enum slice", tt.name)
		}
	}
}

func TestGencontentformat_EffortValuesMatchCanonical(t *testing.T) {
	want := map[string]bool{"low": true, "medium": true, "high": true, "max": true}
	for _, v := range contentformat.Effort {
		if !want[v] {
			t.Errorf("unexpected effort value %q; not in canonical set", v)
		}
		delete(want, v)
	}
	for missing := range want {
		t.Errorf("missing canonical effort value %q", missing)
	}
}

func TestGencontentformat_PermissionModeMatchesCanonical(t *testing.T) {
	// Mirrors the values handled in converter/agents.go switch statements.
	want := map[string]bool{
		"default":           true,
		"acceptEdits":       true,
		"plan":              true,
		"dontAsk":           true,
		"bypassPermissions": true,
	}
	for _, v := range contentformat.PermissionMode {
		if !want[v] {
			t.Errorf("unexpected permission_mode value %q", v)
		}
		delete(want, v)
	}
	for missing := range want {
		t.Errorf("missing canonical permission_mode value %q", missing)
	}
}

// TestGencontentformat_ContentTypeMatchesCatalog verifies the emitted
// content_type list stays in sync with catalog.AllContentTypes(). If this
// fails, either the catalog grew (update contentformat.ContentType) or a
// virtual type leaked in (remove from contentformat.ContentType).
func TestGencontentformat_ContentTypeMatchesCatalog(t *testing.T) {
	catTypes := make(map[string]bool)
	for _, ct := range catalog.AllContentTypes() {
		catTypes[string(ct)] = true
	}

	emitted := make(map[string]bool)
	for _, v := range contentformat.ContentType {
		emitted[v] = true
	}

	for v := range catTypes {
		if !emitted[v] {
			t.Errorf("catalog.AllContentTypes() contains %q but contentformat.ContentType does not", v)
		}
	}
	for v := range emitted {
		if !catTypes[v] {
			t.Errorf("contentformat.ContentType contains %q but catalog.AllContentTypes() does not", v)
		}
	}
}
