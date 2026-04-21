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

// --- Adversarial / edge-case coverage ---

// TestMarkdownExtractor_NoHeadings_NoPrimary verifies extraction of headless
// docs when no Primary selector is given. Landmarks stay empty, but the
// single table's rows are extracted.
func TestMarkdownExtractor_NoHeadings_NoPrimary(t *testing.T) {
	raw := []byte(`| Col1 | Col2 |
|------|------|
| alpha | beta |
| gamma | delta |
`)
	result, err := capmon.Extract(context.Background(), "markdown", raw, capmon.SelectorConfig{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	wantFields := map[string]string{
		"row_0_col_0": "Col1",
		"row_0_col_1": "Col2",
		"row_1_col_0": "alpha",
		"row_1_col_1": "beta",
		"row_2_col_0": "gamma",
		"row_2_col_1": "delta",
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
	if len(result.Landmarks) != 0 {
		t.Errorf("expected no landmarks for heading-less doc, got %v", result.Landmarks)
	}
}

// TestMarkdownExtractor_MismatchedHeadingLevel verifies that a Primary
// selector targeting a heading level that doesn't exist in the document
// produces zero fields — the scoped walk never enters the target section.
// Landmarks are still captured because Heading traversal runs unconditionally.
func TestMarkdownExtractor_MismatchedHeadingLevel(t *testing.T) {
	raw := []byte(`## Events

| Event Name | Handler |
|------------|---------|
| PreToolUse | pre.handler |
| PostToolUse | post.handler |
`)
	// Primary specifies level 3 but the document heading is level 2.
	cfg := capmon.SelectorConfig{Primary: "### Events"}
	result, err := capmon.Extract(context.Background(), "markdown", raw, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(result.Fields) != 0 {
		t.Errorf("mismatched heading level should yield zero fields, got %v", result.Fields)
	}
	foundLandmark := false
	for _, l := range result.Landmarks {
		if l == "Events" {
			foundLandmark = true
		}
	}
	if !foundLandmark {
		t.Errorf("landmark 'Events' should still be captured, got %v", result.Landmarks)
	}
}

// TestMarkdownExtractor_MultiTable_TargetsLaterSection verifies that when
// a doc has multiple tables across sections and Primary targets the second
// heading, only rows from that section are extracted — indexing resets to
// zero on entry, so the Ignored section's contents must not bleed through.
func TestMarkdownExtractor_MultiTable_TargetsLaterSection(t *testing.T) {
	raw := []byte(`## Ignored

| X | Y |
|---|---|
| irrelevant | should-not-appear |

## Target

| A | B |
|---|---|
| kept-a | kept-b |
`)
	result, err := capmon.Extract(context.Background(), "markdown", raw, capmon.SelectorConfig{Primary: "## Target"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	wantFields := map[string]string{
		"row_0_col_0": "A",
		"row_0_col_1": "B",
		"row_1_col_0": "kept-a",
		"row_1_col_1": "kept-b",
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
	// Content from the Ignored section must NOT appear anywhere.
	for _, fv := range result.Fields {
		if strings.Contains(fv.Value, "irrelevant") || strings.Contains(fv.Value, "should-not-appear") {
			t.Errorf("field from Ignored section leaked through: %+v", result.Fields)
			break
		}
	}
}

// TestMarkdownExtractor_UnicodeCells verifies Unicode passes through the
// sanitizer unchanged, including multi-byte characters and emoji.
func TestMarkdownExtractor_UnicodeCells(t *testing.T) {
	raw := []byte(`## Events

| Name | Label |
|------|-------|
| α-release | 🚀 launched |
| café | naïve |
`)
	result, err := capmon.Extract(context.Background(), "markdown", raw, capmon.SelectorConfig{Primary: "## Events"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	wantFields := map[string]string{
		"row_1_col_0": "α-release",
		"row_1_col_1": "🚀 launched",
		"row_2_col_0": "café",
		"row_2_col_1": "naïve",
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

// TestMarkdownExtractor_InlineCodeInCell verifies that cells containing
// inline code (backticks) have the code text extracted without the
// backtick delimiters themselves.
func TestMarkdownExtractor_InlineCodeInCell(t *testing.T) {
	raw := []byte(`## Events

| Event | Handler |
|-------|---------|
| PreToolUse | ` + "`hooks.pre_tool`" + ` |
`)
	result, err := capmon.Extract(context.Background(), "markdown", raw, capmon.SelectorConfig{Primary: "## Events"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	fv, ok := result.Fields["row_1_col_1"]
	if !ok {
		t.Fatalf("expected row_1_col_1, got fields %v", result.Fields)
	}
	if fv.Value != "hooks.pre_tool" {
		t.Errorf("inline code value = %q, want %q (backticks must be stripped, content preserved)", fv.Value, "hooks.pre_tool")
	}
}

// TestMarkdownExtractor_MalformedTable_NoSeparator verifies that pipe-
// delimited rows without the separator row (---|---) are NOT parsed as
// a table by goldmark, so zero row fields appear. This pins the goldmark
// dependency's behaviour.
func TestMarkdownExtractor_MalformedTable_NoSeparator(t *testing.T) {
	raw := []byte(`## Events

| A | B |
| x | y |
`)
	result, err := capmon.Extract(context.Background(), "markdown", raw, capmon.SelectorConfig{Primary: "## Events"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for key := range result.Fields {
		if strings.HasPrefix(key, "row_") {
			t.Errorf("malformed table (no separator) should yield no row fields, got %v", result.Fields)
			break
		}
	}
}

// TestMarkdownExtractor_UnevenColumns verifies that when rows have more or
// fewer cells than the header, goldmark pads with empties and the
// extractor still produces field entries (including empty values) for
// every cell position. Pins the per-row column indexing behaviour.
func TestMarkdownExtractor_UnevenColumns(t *testing.T) {
	raw := []byte(`## Events

| A | B | C |
|---|---|---|
| x | y |
| p | q | r | s |
`)
	result, err := capmon.Extract(context.Background(), "markdown", raw, capmon.SelectorConfig{Primary: "## Events"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Header row: all three columns present.
	wantHeader := map[string]string{"row_0_col_0": "A", "row_0_col_1": "B", "row_0_col_2": "C"}
	for k, v := range wantHeader {
		fv, ok := result.Fields[k]
		if !ok || fv.Value != v {
			t.Errorf("header field %q = %v, want %q", k, result.Fields[k], v)
		}
	}
	// Row 1: short by one cell — GFM pads position 2 with empty string.
	if fv, ok := result.Fields["row_1_col_2"]; !ok {
		t.Errorf("row_1_col_2 should exist as padded empty cell, got fields %v", result.Fields)
	} else if fv.Value != "" {
		t.Errorf("row_1_col_2 padding = %q, want empty", fv.Value)
	}
	// Row 2: extra cell at position 3. Assertion depends on whether
	// goldmark preserves overflow; verify both data cells we care about.
	if fv, ok := result.Fields["row_2_col_2"]; !ok || fv.Value != "r" {
		t.Errorf("row_2_col_2 = %v, want \"r\"", result.Fields["row_2_col_2"])
	}
}

// TestMarkdownExtractor_NoHeadings_WithPrimary verifies that when Primary
// is set but the document has no headings at all, extraction produces zero
// fields — the scoped walk never sees a matching heading to enter.
func TestMarkdownExtractor_NoHeadings_WithPrimary(t *testing.T) {
	raw := []byte(`| A | B |
|---|---|
| x | y |
`)
	result, err := capmon.Extract(context.Background(), "markdown", raw, capmon.SelectorConfig{Primary: "## Events"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(result.Fields) != 0 {
		t.Errorf("headless doc with Primary should yield zero fields, got %v", result.Fields)
	}
}
