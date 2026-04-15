package capmon_test

import (
	"bytes"
	"context"
	"fmt"
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

func TestFindOpenCapmonIssue_Found(t *testing.T) {
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		return []byte(`[{"number":42,"body":"<!-- capmon-check: test-provider/skills -->\nsome issue body"}]`), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	num, found, err := capmon.FindOpenCapmonIssue("test-provider", "skills")
	if err != nil {
		t.Fatalf("FindOpenCapmonIssue: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if num != 42 {
		t.Errorf("issue number = %d, want 42", num)
	}
}

func TestFindOpenCapmonIssue_NotFound(t *testing.T) {
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	num, found, err := capmon.FindOpenCapmonIssue("test-provider", "skills")
	if err != nil {
		t.Fatalf("FindOpenCapmonIssue: %v", err)
	}
	if found {
		t.Errorf("expected found=false, got issue number %d", num)
	}
}

func TestFindOpenCapmonIssue_WrongAnchor(t *testing.T) {
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		// Issue exists but has a different anchor (different content type).
		return []byte(`[{"number":7,"body":"<!-- capmon-check: test-provider/hooks -->\nbody"}]`), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	_, found, err := capmon.FindOpenCapmonIssue("test-provider", "skills")
	if err != nil {
		t.Fatalf("FindOpenCapmonIssue: %v", err)
	}
	if found {
		t.Error("expected found=false when anchor does not match content type")
	}
}

func TestCreateCapmonChangeIssue_Success(t *testing.T) {
	var capturedBody string
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		for i, a := range args {
			if a == "--body" && i+1 < len(args) {
				capturedBody = args[i+1]
			}
		}
		// gh issue create prints the issue URL (no --json flag).
		return []byte("https://github.com/test/repo/issues/99\n"), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	num, err := capmon.CreateCapmonChangeIssue(context.Background(), "test-provider", "skills", "Change detected", "diff content here")
	if err != nil {
		t.Fatalf("CreateCapmonChangeIssue: %v", err)
	}
	if num != 99 {
		t.Errorf("issue number = %d, want 99", num)
	}
	anchor := "<!-- capmon-check: test-provider/skills -->"
	if !strings.Contains(capturedBody, anchor) {
		t.Errorf("body should contain anchor comment %q, got: %q", anchor, capturedBody)
	}
	if !strings.Contains(capturedBody, "diff content here") {
		t.Errorf("body should contain issue body text, got: %q", capturedBody)
	}
}

func TestCreateCapmonChangeIssue_InvalidSlug(t *testing.T) {
	_, err := capmon.CreateCapmonChangeIssue(context.Background(), "INVALID SLUG", "skills", "title", "body")
	if err == nil {
		t.Error("expected error for invalid slug")
	}
}

func TestAppendCapmonChangeEvent_Success(t *testing.T) {
	var capturedArgs []string
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	err := capmon.AppendCapmonChangeEvent(context.Background(), 42, "event body text")
	if err != nil {
		t.Fatalf("AppendCapmonChangeEvent: %v", err)
	}
	// Verify gh was called with "issue comment 42"
	if len(capturedArgs) < 3 {
		t.Fatalf("expected at least 3 args, got %v", capturedArgs)
	}
	if capturedArgs[0] != "issue" || capturedArgs[1] != "comment" || capturedArgs[2] != "42" {
		t.Errorf("expected 'issue comment 42', got %v", capturedArgs[:3])
	}
	found := false
	for i, a := range capturedArgs {
		if a == "--body" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "event body text" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --body 'event body text' in args: %v", capturedArgs)
	}
}

func TestFindOpenCapmonWarningIssue_Found(t *testing.T) {
	w := capmon.ValidationWarning{
		File:  "/tmp/test.yaml",
		Field: "content_types.skills.provider_extensions[model].value_type",
		Value: "badtype",
	}
	anchor := fmt.Sprintf("<!-- capmon-warn: %s -->", w.DeduplicationKey())
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		return []byte(fmt.Sprintf(`[{"number":55,"body":"%s\nsome body"}]`, anchor)), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	num, found, err := capmon.FindOpenCapmonWarningIssue("test-provider", w)
	if err != nil {
		t.Fatalf("FindOpenCapmonWarningIssue: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if num != 55 {
		t.Errorf("issue number = %d, want 55", num)
	}
}

func TestFindOpenCapmonWarningIssue_NotFound(t *testing.T) {
	w := capmon.ValidationWarning{
		File:  "/tmp/test.yaml",
		Field: "value_type",
		Value: "badtype",
	}
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	_, found, err := capmon.FindOpenCapmonWarningIssue("test-provider", w)
	if err != nil {
		t.Fatalf("FindOpenCapmonWarningIssue: %v", err)
	}
	if found {
		t.Error("expected found=false")
	}
}

func TestCreateCapmonWarningIssue_Success(t *testing.T) {
	w := capmon.ValidationWarning{
		File:    "/tmp/test.yaml",
		Field:   "value_type",
		Value:   "badtype",
		Message: "not in allow-list",
	}
	var capturedTitle, capturedBody string
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		for i, a := range args {
			if a == "--title" && i+1 < len(args) {
				capturedTitle = args[i+1]
			}
			if a == "--body" && i+1 < len(args) {
				capturedBody = args[i+1]
			}
		}
		return []byte("https://github.com/test/repo/issues/77\n"), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	num, err := capmon.CreateCapmonWarningIssue(context.Background(), "test-provider", w)
	if err != nil {
		t.Fatalf("CreateCapmonWarningIssue: %v", err)
	}
	if num != 77 {
		t.Errorf("issue number = %d, want 77", num)
	}
	if !strings.Contains(capturedTitle, "capmon-warn-") {
		t.Errorf("title should contain dedup key prefix, got: %q", capturedTitle)
	}
	if !strings.Contains(capturedBody, "capmon-warn:") {
		t.Errorf("body should contain anchor comment, got: %q", capturedBody)
	}
	if !strings.Contains(capturedBody, "badtype") {
		t.Errorf("body should contain offending value, got: %q", capturedBody)
	}
}

func TestCreateCapmonWarningIssue_InvalidSlug(t *testing.T) {
	w := capmon.ValidationWarning{Field: "x", Value: "y"}
	_, err := capmon.CreateCapmonWarningIssue(context.Background(), "INVALID SLUG", w)
	if err == nil {
		t.Error("expected error for invalid slug")
	}
}

func TestCloseResolvedWarningIssues_ClosesStale(t *testing.T) {
	var closedNumbers []string
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "issue" && args[1] == "list" {
			return []byte(`[
				{"number":10,"body":"<!-- capmon-warn: aaaa1111bbbb2222 -->\nstale warning"},
				{"number":20,"body":"<!-- capmon-warn: cccc3333dddd4444 -->\nstill active"}
			]`), nil
		}
		if len(args) >= 2 && args[0] == "issue" && args[1] == "close" {
			closedNumbers = append(closedNumbers, args[2])
			return []byte(""), nil
		}
		return []byte(""), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	// Only cccc3333dddd4444 is seen — aaaa1111bbbb2222 should be closed.
	seenKeys := map[string]bool{"cccc3333dddd4444": true}
	err := capmon.CloseResolvedWarningIssues(context.Background(), "test-provider", seenKeys)
	if err != nil {
		t.Fatalf("CloseResolvedWarningIssues: %v", err)
	}
	if len(closedNumbers) != 1 || closedNumbers[0] != "10" {
		t.Errorf("expected to close issue 10, closed: %v", closedNumbers)
	}
}

func TestCloseResolvedWarningIssues_NoneToClose(t *testing.T) {
	var closeCalled bool
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "issue" && args[1] == "close" {
			closeCalled = true
		}
		if len(args) >= 2 && args[0] == "issue" && args[1] == "list" {
			return []byte(`[{"number":10,"body":"<!-- capmon-warn: aaaa1111bbbb2222 -->\nwarning"}]`), nil
		}
		return []byte(""), nil
	})
	defer capmon.SetGHCommandForTest(nil)

	// Key is still seen — nothing to close.
	seenKeys := map[string]bool{"aaaa1111bbbb2222": true}
	err := capmon.CloseResolvedWarningIssues(context.Background(), "test-provider", seenKeys)
	if err != nil {
		t.Fatalf("CloseResolvedWarningIssues: %v", err)
	}
	if closeCalled {
		t.Error("should not close any issues when all keys are still seen")
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
