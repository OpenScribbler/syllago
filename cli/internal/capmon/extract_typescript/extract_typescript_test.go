//go:build cgo

package extract_typescript_test

import (
	"context"
	"os"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_typescript"
)

func TestTypeScriptExtractor_EnumStringValues(t *testing.T) {
	raw, err := os.ReadFile("../testdata/fixtures/typescript/hooks.ts")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cfg := capmon.SelectorConfig{MinResults: 3}
	result, err := capmon.Extract(context.Background(), "typescript", raw, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Enum string values must be present
	wantValues := []string{"PreToolUse", "PostToolUse", "Stop", "BashTool"}
	for _, want := range wantValues {
		found := false
		for _, fv := range result.Fields {
			if fv.Value == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected value %q in extracted fields; got %v", want, result.Fields)
		}
	}
}

func TestTypeScriptExtractor_ConstStringValues(t *testing.T) {
	raw, err := os.ReadFile("../testdata/fixtures/typescript/hooks.ts")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "typescript", raw, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// HOOK_VERSION const must be extracted
	found := false
	for _, fv := range result.Fields {
		if fv.Value == "2024.1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected const string value %q in extracted fields", "2024.1")
	}
}

func TestTypeScriptExtractor_Landmarks(t *testing.T) {
	raw, err := os.ReadFile("../testdata/fixtures/typescript/hooks.ts")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := capmon.Extract(context.Background(), "typescript", raw, capmon.SelectorConfig{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Top-level type/enum/interface names are landmarks
	wantLandmarks := []string{"HookEvent", "ToolName", "HookConfig"}
	for _, want := range wantLandmarks {
		found := false
		for _, got := range result.Landmarks {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected landmark %q in %v", want, result.Landmarks)
		}
	}
}

func TestTypeScriptExtractor_BareEnumMembers(t *testing.T) {
	// Bare enum members (no = "value") should use the member name as value
	raw := []byte(`export enum Direction { Up, Down, Left, Right }`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "typescript", raw, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Bare members should appear as fields with name-as-value
	found := false
	for k, fv := range result.Fields {
		if (k == "Direction.Up" || k == "Up") && fv.Value == "Up" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("bare enum member 'Up' not found in fields: %v", result.Fields)
	}
}

func TestTypeScriptExtractor_EnumWithNumbers(t *testing.T) {
	// Enum members with numeric values — the extractor must stringify each
	// numeric literal and preserve the enum-member-to-value mapping so that
	// a regression that swapped, dropped, or truncated values would fail.
	raw := []byte(`export enum Status { OK = 200, NotFound = 404, Error = 500 }`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "typescript", raw, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Enum name surfaces as a landmark.
	foundLandmark := false
	for _, l := range result.Landmarks {
		if l == "Status" {
			foundLandmark = true
			break
		}
	}
	if !foundLandmark {
		t.Errorf("expected 'Status' landmark, got %v", result.Landmarks)
	}

	// Each member must stringify to its numeric value, keyed <enum>.<member>.
	wantFields := map[string]string{
		"Status.OK":       "200",
		"Status.NotFound": "404",
		"Status.Error":    "500",
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

func TestTypeScriptExtractor_InterfaceProperties(t *testing.T) {
	raw := []byte(`export interface Skill {
  name: string;
  description: string;
  filePath: string;
}`)
	result, err := capmon.Extract(context.Background(), "typescript", raw, capmon.SelectorConfig{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	foundLandmark := false
	for _, l := range result.Landmarks {
		if l == "Skill" {
			foundLandmark = true
			break
		}
	}
	if !foundLandmark {
		t.Errorf("expected 'Skill' in landmarks, got %v", result.Landmarks)
	}
	wantFields := map[string]string{
		"Skill.name":        "name",
		"Skill.description": "description",
		"Skill.filePath":    "filePath",
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

func TestTypeScriptExtractor_InterfaceFromFixture(t *testing.T) {
	raw, err := os.ReadFile("../testdata/fixtures/typescript/hooks.ts")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := capmon.Extract(context.Background(), "typescript", raw, capmon.SelectorConfig{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	wantFields := []string{"HookConfig.event", "HookConfig.blocking", "HookConfig.command"}
	for _, key := range wantFields {
		if _, ok := result.Fields[key]; !ok {
			t.Errorf("expected field %q from HookConfig interface, not found", key)
		}
	}
}

func TestTypeScriptExtractor_AnchorMissing(t *testing.T) {
	raw := []byte(`export enum Foo { Bar = "Bar" }`)
	cfg := capmon.SelectorConfig{ExpectedContains: "NonExistentAnchor"}
	_, err := capmon.Extract(context.Background(), "typescript", raw, cfg)
	if err == nil {
		t.Error("expected error when anchor is missing")
	}
}

func TestTypeScriptExtractor_ParseError(t *testing.T) {
	// tree-sitter is tolerant of syntax errors but produces partial results
	// — not a hard parse error. Instead test empty input.
	raw := []byte{}
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "typescript", raw, cfg)
	if err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
	if len(result.Fields) != 0 {
		t.Errorf("expected 0 fields from empty input, got %d", len(result.Fields))
	}
}

func TestTypeScriptExtractor_Partial(t *testing.T) {
	raw := []byte(`export const X = "hello"`)
	cfg := capmon.SelectorConfig{MinResults: 100}
	result, err := capmon.Extract(context.Background(), "typescript", raw, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !result.Partial {
		t.Error("expected Partial=true when fewer fields than MinResults")
	}
}
