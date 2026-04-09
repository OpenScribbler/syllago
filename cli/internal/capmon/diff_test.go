package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestDiffProviderCapabilities_NoChange(t *testing.T) {
	extracted := &capmon.ExtractedSource{
		Fields: map[string]capmon.FieldValue{
			"hooks.events.before_tool_execute.native_name": {Value: "PreToolUse"},
		},
	}
	current := map[string]string{
		"hooks.events.before_tool_execute.native_name": "PreToolUse",
	}
	diff := capmon.DiffProviderCapabilities("claude-code", "run-001", extracted, current)
	if len(diff.Changes) != 0 {
		t.Errorf("expected no changes, got %d", len(diff.Changes))
	}
}

func TestDiffProviderCapabilities_FieldChanged(t *testing.T) {
	extracted := &capmon.ExtractedSource{
		Fields: map[string]capmon.FieldValue{
			"hooks.events.before_tool_execute.native_name": {Value: "PreTool"},
		},
	}
	current := map[string]string{
		"hooks.events.before_tool_execute.native_name": "PreToolUse",
	}
	diff := capmon.DiffProviderCapabilities("claude-code", "run-001", extracted, current)
	if len(diff.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(diff.Changes))
	}
	if diff.Changes[0].OldValue != "PreToolUse" {
		t.Errorf("OldValue: got %q, want %q", diff.Changes[0].OldValue, "PreToolUse")
	}
	if diff.Changes[0].NewValue != "PreTool" {
		t.Errorf("NewValue: got %q, want %q", diff.Changes[0].NewValue, "PreTool")
	}
}

func TestDiff_NoChange(t *testing.T) {
	extracted := &capmon.ExtractedSource{
		Fields: map[string]capmon.FieldValue{
			"hooks.events.before_tool_execute.native_name": {Value: "PreToolUse"},
		},
		Landmarks: []string{"Events", "Configuration"},
	}
	current := map[string]string{
		"hooks.events.before_tool_execute.native_name": "PreToolUse",
	}
	diff := capmon.DiffProviderCapabilities("claude-code", "run-001", extracted, current)
	if len(diff.Changes) != 0 {
		t.Errorf("expected no changes, got %d", len(diff.Changes))
	}
}

func TestDiff_FieldChanged(t *testing.T) {
	extracted := &capmon.ExtractedSource{
		Fields: map[string]capmon.FieldValue{
			"hooks.events.before_tool_execute.native_name": {Value: "PreTool"},
		},
		Landmarks: []string{"Events", "Configuration"},
	}
	current := map[string]string{
		"hooks.events.before_tool_execute.native_name": "PreToolUse",
	}
	diff := capmon.DiffProviderCapabilities("claude-code", "run-001", extracted, current)
	if len(diff.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(diff.Changes), diff.Changes)
	}
}

func TestDiff_StructuralDrift_NewHeading(t *testing.T) {
	// Field-level version: new extracted field not in current → structural drift
	extracted := &capmon.ExtractedSource{
		Fields: map[string]capmon.FieldValue{
			"hooks.events.before_tool_execute.native_name": {Value: "PreToolUse"},
			"hooks.events.new_event.native_name":           {Value: "NewEvent"},
		},
	}
	current := map[string]string{
		"hooks.events.before_tool_execute.native_name": "PreToolUse",
	}
	diff := capmon.DiffProviderCapabilities("claude-code", "run-001", extracted, current)
	if len(diff.StructuralDrift) == 0 {
		t.Error("expected structural drift for new field not in YAML")
	}
}

func TestDiff_StructuralDrift_NewHeading_Landmarks(t *testing.T) {
	// Landmark version: new heading in extracted not in knownLandmarks → structural drift
	extracted := &capmon.ExtractedSource{
		Fields:    map[string]capmon.FieldValue{},
		Landmarks: []string{"Events", "New Section"},
	}
	knownLandmarks := []string{"Events"}
	diff := capmon.DiffLandmarks("claude-code", "run-001", extracted.Landmarks, knownLandmarks)
	if len(diff.StructuralDrift) != 1 {
		t.Fatalf("expected 1 structural drift entry, got %d: %v", len(diff.StructuralDrift), diff.StructuralDrift)
	}
	if diff.StructuralDrift[0] != "New Section" {
		t.Errorf("StructuralDrift[0]: got %q, want %q", diff.StructuralDrift[0], "New Section")
	}
}
