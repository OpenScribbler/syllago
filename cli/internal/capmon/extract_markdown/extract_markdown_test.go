package extract_markdown_test

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_markdown"
)

func TestMarkdownExtractor_HeadingPath(t *testing.T) {
	raw := []byte(`# Top Level

## Events

| Event Name | Description |
|------------|-------------|
| PreToolUse | Fires before tool |
| PostToolUse | Fires after tool |

## Other Section

| Other | Data |
|-------|------|
| foo | bar |
`)
	cfg := capmon.SelectorConfig{
		Primary:          "## Events",
		ExpectedContains: "Event Name",
		MinResults:       1,
	}
	result, err := capmon.Extract(context.Background(), "markdown", raw, cfg)
	if err != nil {
		t.Fatalf("Extract markdown: %v", err)
	}
	found := false
	for _, fv := range result.Fields {
		if fv.Value == "PreToolUse" {
			found = true
		}
	}
	if !found {
		t.Error("expected PreToolUse in extracted fields")
	}
	for _, fv := range result.Fields {
		if fv.Value == "foo" {
			t.Error("field 'foo' from Other Section should not be extracted")
		}
	}
}

func TestMarkdownExtractor_Landmarks(t *testing.T) {
	raw := []byte(`# Top Level

## Events

## Configuration

### Sub-section
`)
	cfg := capmon.SelectorConfig{Primary: "## Events"}
	result, err := capmon.Extract(context.Background(), "markdown", raw, cfg)
	if err != nil {
		t.Fatalf("Extract markdown: %v", err)
	}
	wantLandmarks := []string{"Top Level", "Events", "Configuration", "Sub-section"}
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
}

func TestMarkdownExtractor_AnchorMissing(t *testing.T) {
	raw := []byte(`## Events

| Event Name | Desc |
|------------|------|
| PreToolUse | x |
`)
	cfg := capmon.SelectorConfig{
		Primary:          "## Events",
		ExpectedContains: "NonExistentAnchor",
	}
	_, err := capmon.Extract(context.Background(), "markdown", raw, cfg)
	if err == nil {
		t.Error("expected error for missing anchor")
	}
	if !strings.Contains(err.Error(), "anchor_missing") {
		t.Errorf("error %q should contain anchor_missing", err.Error())
	}
}
