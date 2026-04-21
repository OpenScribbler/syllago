package capmon_test

import (
	"context"
	"os"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_html"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_yaml"
)

func TestFixtures_ClaudeCodeHooksHTML(t *testing.T) {
	raw, err := os.ReadFile("testdata/fixtures/claude-code/hooks-docs.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cfg := capmon.SelectorConfig{
		Primary:          "main table tr td:first-child",
		Fallback:         "main table",
		ExpectedContains: "Event Name",
		MinResults:       6,
	}
	result, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(result.Fields) < 6 {
		t.Errorf("expected at least 6 fields, got %d", len(result.Fields))
	}
	if result.Partial {
		t.Error("result should not be partial with real fixture")
	}
	// Verify known events are present
	wantValues := []string{"PreToolUse", "PostToolUse", "Stop"}
	for _, want := range wantValues {
		found := false
		for _, fv := range result.Fields {
			if fv.Value == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected event %q in extracted fields", want)
		}
	}
}

func TestFixtures_WindsurfLLMSTxt(t *testing.T) {
	raw, err := os.ReadFile("testdata/fixtures/windsurf/llms-full.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cfg := capmon.SelectorConfig{
		MinResults: 3,
	}
	result, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
	if err != nil {
		t.Fatalf("Extract yaml from windsurf fixture: %v", err)
	}
	if len(result.Fields) == 0 {
		t.Error("expected fields from windsurf fixture, got 0")
	}
	// Landmarks should include top-level sections
	wantLandmarks := []string{"hooks", "tools"}
	for _, want := range wantLandmarks {
		found := false
		for _, got := range result.Landmarks {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("landmark %q not found in %v", want, result.Landmarks)
		}
	}

	// Field-value assertions — the extractor must preserve both the nested
	// key path and the scalar value for every leaf. A regression that
	// flattened keys incorrectly, mapped values to wrong events, or
	// stringified booleans as something other than "true"/"false" would
	// fail these checks even if len(result.Fields) stayed non-zero.
	wantFields := map[string]string{
		"hooks.PreToolUse.description":  "Fires before any tool use",
		"hooks.PreToolUse.blocking":     "true",
		"hooks.PostToolUse.description": "Fires after any tool use",
		"hooks.PostToolUse.blocking":    "false",
		"hooks.Stop.description":        "Fires when generation stops",
		"hooks.Stop.blocking":           "false",
		"tools.shell.native":            "BashTool",
		"tools.file_read.native":        "ReadTool",
		"tools.file_write.native":       "WriteTool",
		"tools.file_edit.native":        "EditTool",
	}
	for key, wantValue := range wantFields {
		fv, ok := result.Fields[key]
		if !ok {
			t.Errorf("expected field %q, not found in %v", key, result.Fields)
			continue
		}
		if fv.Value != wantValue {
			t.Errorf("field %q: value = %q, want %q", key, fv.Value, wantValue)
		}
	}
}

func TestFixtures_LiveNetwork(t *testing.T) {
	if os.Getenv("SYLLAGO_TEST_NETWORK") == "" {
		t.Skip("set SYLLAGO_TEST_NETWORK=1 to run live network tests")
	}
	// Live test placeholder — requires network access and valid HTTPS sources
	t.Log("live network test would fetch from real provider URLs")
}
