//go:build cgo

package extract_rust_test

import (
	"context"
	"os"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_rust"
)

func TestRustExtractor_EnumVariantNames(t *testing.T) {
	raw, err := os.ReadFile("../testdata/fixtures/rust/hooks.rs")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cfg := capmon.SelectorConfig{MinResults: 3}
	result, err := capmon.Extract(context.Background(), "rust", raw, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Enum variant names must be present as values
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
			t.Errorf("expected variant %q in extracted fields; got %v", want, result.Fields)
		}
	}
}

func TestRustExtractor_ConstStringValues(t *testing.T) {
	raw, err := os.ReadFile("../testdata/fixtures/rust/hooks.rs")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := capmon.Extract(context.Background(), "rust", raw, capmon.SelectorConfig{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// HOOK_VERSION string const must be extracted
	found := false
	for _, fv := range result.Fields {
		if fv.Value == "2024.1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected const string value %q in fields", "2024.1")
	}
}

func TestRustExtractor_Landmarks(t *testing.T) {
	raw, err := os.ReadFile("../testdata/fixtures/rust/hooks.rs")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := capmon.Extract(context.Background(), "rust", raw, capmon.SelectorConfig{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Top-level struct/enum/trait names are landmarks
	wantLandmarks := []string{"HookEvent", "ToolName", "HookConfig", "HookHandler"}
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

func TestRustExtractor_AnchorMissing(t *testing.T) {
	raw := []byte(`pub enum Foo { Bar }`)
	cfg := capmon.SelectorConfig{ExpectedContains: "NonExistentAnchor"}
	_, err := capmon.Extract(context.Background(), "rust", raw, cfg)
	if err == nil {
		t.Error("expected error when anchor is missing")
	}
}

func TestRustExtractor_EmptyInput(t *testing.T) {
	raw := []byte{}
	result, err := capmon.Extract(context.Background(), "rust", raw, capmon.SelectorConfig{})
	if err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
	if len(result.Fields) != 0 {
		t.Errorf("expected 0 fields from empty input, got %d", len(result.Fields))
	}
}

func TestRustExtractor_Partial(t *testing.T) {
	raw := []byte(`pub const X: &str = "hello";`)
	cfg := capmon.SelectorConfig{MinResults: 100}
	result, err := capmon.Extract(context.Background(), "rust", raw, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !result.Partial {
		t.Error("expected Partial=true when fewer fields than MinResults")
	}
}
