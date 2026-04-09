package capmon_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"claude-code", false},
		{"gemini-cli", false},
		{"windsurf", false},
		{"UPPER", true},
		{"has space", true},
		{"-leading-dash", true},
		{"trailing-dash-", true},
		{"a", true}, // single char fails [a-z0-9][a-z0-9-]*[a-z0-9] pattern
		{"ab", false},
		{"../escape", true},
		{"capmon/drift", true}, // slash not allowed
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := capmon.SanitizeSlug(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeSlug(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err == nil && got != tt.input {
				t.Errorf("SanitizeSlug(%q) = %q, want same as input", tt.input, got)
			}
		})
	}
}

func TestBuildPRBody_NoTemplateInjection(t *testing.T) {
	diff := capmon.CapabilityDiff{
		Provider: "test-provider",
		RunID:    "run-001",
		Changes: []capmon.FieldChange{
			{
				FieldPath: "hooks.events.before_tool_execute.native_name",
				OldValue:  "{{.Secret}}", // template injection attempt
				NewValue:  "PreToolUse",
			},
		},
	}
	var buf bytes.Buffer
	err := capmon.BuildPRBody(&buf, diff)
	if err != nil {
		t.Fatalf("BuildPRBody returned error: %v", err)
	}
	body := buf.String()
	if !strings.Contains(body, "```") {
		t.Error("PR body must fence extracted values with triple backticks")
	}
	if !strings.Contains(body, "{{.Secret}}") {
		t.Error("template injection attempt must appear verbatim in PR body")
	}
	if !strings.Contains(body, "run-001") {
		t.Error("RunID must appear in PR body")
	}
	if !strings.Contains(body, "Pipeline output is not ground truth") {
		t.Error("fixed footer disclaimer must be present")
	}
}
