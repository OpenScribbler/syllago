package capmon_test

import (
	"bytes"
	"context"
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

func TestDeduplicatePR_NoneExists(t *testing.T) {
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil // empty PR list
	})
	defer capmon.SetGHCommandForTest(nil)

	url, found, err := capmon.DeduplicatePR(context.Background(), "test-provider")
	if err != nil {
		t.Fatalf("DeduplicatePR: %v", err)
	}
	if found {
		t.Errorf("expected found=false when no open PRs, got url=%q", url)
	}
}

func TestRecordConsecutiveFailure_ThirdFailure(t *testing.T) {
	cacheDir := t.TempDir()
	// First two failures — no issue
	for i := 0; i < 2; i++ {
		if err := capmon.RecordConsecutiveFailure(context.Background(), cacheDir, "test-provider"); err != nil {
			t.Fatalf("failure %d: %v", i+1, err)
		}
	}
	// Third failure — should trigger issue creation attempt
	issueCreated := false
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "issue" {
				issueCreated = true
			}
		}
		return []byte("https://github.com/test/repo/issues/1"), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	if err := capmon.RecordConsecutiveFailure(context.Background(), cacheDir, "test-provider"); err != nil {
		t.Fatalf("third failure: %v", err)
	}
	if !issueCreated {
		t.Error("expected GitHub issue to be created on 3rd consecutive failure")
	}
}

func TestGHRunner_UsesOverride(t *testing.T) {
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		return []byte("stub-output"), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	out, err := capmon.GHRunner("version")
	if err != nil {
		t.Fatalf("GHRunner: %v", err)
	}
	if string(out) != "stub-output" {
		t.Errorf("GHRunner output = %q, want stub-output", string(out))
	}
}

func TestCreateDriftPR_Success(t *testing.T) {
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		return []byte("https://github.com/test/repo/pull/42\n"), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	diff := capmon.CapabilityDiff{Provider: "test-provider", RunID: "run-001"}
	url, err := capmon.CreateDriftPR(context.Background(), "test-provider", "run-001", diff)
	if err != nil {
		t.Fatalf("CreateDriftPR: %v", err)
	}
	if url != "https://github.com/test/repo/pull/42" {
		t.Errorf("url = %q, want stripped URL", url)
	}
}

func TestCreateDriftPR_InvalidSlug(t *testing.T) {
	diff := capmon.CapabilityDiff{}
	_, err := capmon.CreateDriftPR(context.Background(), "INVALID SLUG", "run-001", diff)
	if err == nil {
		t.Error("expected error for invalid slug")
	}
}

func TestCreateStructuralIssue_Success(t *testing.T) {
	issueCreated := false
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "issue" {
				issueCreated = true
			}
		}
		return []byte("https://github.com/test/repo/issues/7\n"), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	err := capmon.CreateStructuralIssue(context.Background(), "test-provider", "run-001", []string{"new-section"})
	if err != nil {
		t.Fatalf("CreateStructuralIssue: %v", err)
	}
	if !issueCreated {
		t.Error("expected issue creation gh call")
	}
}

func TestCreateStructuralIssue_InvalidSlug(t *testing.T) {
	err := capmon.CreateStructuralIssue(context.Background(), "INVALID", "run-001", nil)
	if err == nil {
		t.Error("expected error for invalid slug")
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
