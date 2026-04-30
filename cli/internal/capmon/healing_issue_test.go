package capmon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealFailureAnchor(t *testing.T) {
	got := HealFailureAnchor("claude-code", "skills", 2)
	want := "<!-- capmon-heal-fail: claude-code/skills/2 -->"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRecordConsecutiveHealFailure_UnderThreshold(t *testing.T) {
	cacheRoot := t.TempDir()
	ghCalled := 0
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		ghCalled++
		return nil, nil
	})
	defer SetGHCommandForTest(nil)

	issueNum, err := RecordConsecutiveHealFailure(cacheRoot, "claude-code", "skills", 0, &HealResult{FailReason: "variant: all 404"})
	if err != nil {
		t.Fatalf("RecordConsecutiveHealFailure: %v", err)
	}
	if issueNum != 0 {
		t.Errorf("issueNum = %d, want 0 under threshold", issueNum)
	}
	if ghCalled != 0 {
		t.Errorf("gh was called %d times under threshold; should be 0", ghCalled)
	}
}

func TestRecordConsecutiveHealFailure_HitsThreshold_CreatesIssue(t *testing.T) {
	cacheRoot := t.TempDir()

	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if args[0] == "issue" && args[1] == "list" {
			return []byte(`[]`), nil
		}
		if args[0] == "issue" && args[1] == "create" {
			return []byte("https://github.com/org/repo/issues/77\n"), nil
		}
		t.Errorf("unexpected gh: %v", args)
		return nil, nil
	})
	defer SetGHCommandForTest(nil)

	// Prime counter to (threshold - 1) so the next call crosses the line.
	prev := healFailureCountFile(cacheRoot, "claude-code", "skills", 0)
	_ = os.MkdirAll(filepath.Dir(prev), 0755)
	_ = os.WriteFile(prev, []byte("1"), 0644) // healFailureThreshold is 2

	issueNum, err := RecordConsecutiveHealFailure(cacheRoot, "claude-code", "skills", 0, &HealResult{
		FailReason: "github-rename: no candidates",
		CandidateOutcomes: []CandidateOutcome{
			{URL: "https://example.com/missing.md", Strategy: "github-rename", Outcome: OutcomeHTTPError, StatusCode: 404, Detail: "status 404"},
		},
	})
	if err != nil {
		t.Fatalf("RecordConsecutiveHealFailure: %v", err)
	}
	if issueNum != 77 {
		t.Errorf("issueNum = %d, want 77", issueNum)
	}
}

func TestRecordConsecutiveHealFailure_ExistingIssueAppends(t *testing.T) {
	cacheRoot := t.TempDir()
	anchor := HealFailureAnchor("claude-code", "skills", 0)
	var commentedOn int
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if args[0] == "issue" && args[1] == "list" {
			return []byte(`[{"number": 42, "body": "old ` + anchor + ` body"}]`), nil
		}
		if args[0] == "issue" && args[1] == "comment" {
			commentedOn = 42
			return nil, nil
		}
		if args[0] == "issue" && args[1] == "create" {
			t.Error("should not create new issue when one already exists")
			return nil, nil
		}
		return nil, nil
	})
	defer SetGHCommandForTest(nil)

	prev := healFailureCountFile(cacheRoot, "claude-code", "skills", 0)
	_ = os.MkdirAll(filepath.Dir(prev), 0755)
	_ = os.WriteFile(prev, []byte("2"), 0644) // already at threshold

	issueNum, err := RecordConsecutiveHealFailure(cacheRoot, "claude-code", "skills", 0, &HealResult{FailReason: "redirect: still 404"})
	if err != nil {
		t.Fatalf("RecordConsecutiveHealFailure: %v", err)
	}
	if issueNum != 42 {
		t.Errorf("issueNum = %d, want 42", issueNum)
	}
	if commentedOn != 42 {
		t.Errorf("expected comment to be appended to issue 42; got %d", commentedOn)
	}
}

func TestResolveHealFailure_ClosesOpenIssueAndClearsCounter(t *testing.T) {
	cacheRoot := t.TempDir()
	anchor := HealFailureAnchor("claude-code", "skills", 0)

	// Seed counter file so we can verify it's removed.
	counterPath := healFailureCountFile(cacheRoot, "claude-code", "skills", 0)
	_ = os.MkdirAll(filepath.Dir(counterPath), 0755)
	if err := os.WriteFile(counterPath, []byte("5"), 0644); err != nil {
		t.Fatal(err)
	}

	var closedNum int
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if args[0] == "issue" && args[1] == "list" {
			return []byte(`[{"number": 99, "body": "old ` + anchor + `"}]`), nil
		}
		if args[0] == "issue" && args[1] == "close" {
			closedNum = 99
			return nil, nil
		}
		return nil, nil
	})
	defer SetGHCommandForTest(nil)

	if err := ResolveHealFailure(cacheRoot, "claude-code", "skills", 0); err != nil {
		t.Fatalf("ResolveHealFailure: %v", err)
	}
	if _, err := os.Stat(counterPath); !os.IsNotExist(err) {
		t.Errorf("counter file still exists: %v", err)
	}
	if closedNum != 99 {
		t.Errorf("closed = %d, want 99", closedNum)
	}
}

func TestResolveHealFailure_NoOpenIssue(t *testing.T) {
	cacheRoot := t.TempDir()
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if args[0] == "issue" && args[1] == "list" {
			return []byte(`[]`), nil
		}
		if args[0] == "issue" && args[1] == "close" {
			t.Error("close should not be called when no open issue exists")
			return nil, nil
		}
		return nil, nil
	})
	defer SetGHCommandForTest(nil)

	if err := ResolveHealFailure(cacheRoot, "claude-code", "skills", 0); err != nil {
		t.Fatalf("ResolveHealFailure: %v", err)
	}
}

func TestCreateHealFailureIssue_BodyContainsAnchor(t *testing.T) {
	var capturedBody string
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		for i, a := range args {
			if a == "--body" && i+1 < len(args) {
				capturedBody = args[i+1]
			}
		}
		return []byte("https://github.com/org/repo/issues/1\n"), nil
	})
	defer SetGHCommandForTest(nil)

	_, err := createHealFailureIssue("claude-code", "skills", 0, 2, &HealResult{FailReason: "variant: nothing worked"})
	if err != nil {
		t.Fatalf("createHealFailureIssue: %v", err)
	}
	if !strings.Contains(capturedBody, "capmon-heal-fail: claude-code/skills/0") {
		t.Errorf("body missing anchor: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "variant: nothing worked") {
		t.Errorf("body missing failure reason: %s", capturedBody)
	}
}

func TestCreateHealFailureIssue_BodyContainsCandidatesTable(t *testing.T) {
	// Multi-candidate failures must render the diagnostic table in the
	// issue body so reviewers see what was probed before escalation. Same
	// rendering as PR bodies — single source of truth via RenderCandidatesTable.
	var capturedBody string
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		for i, a := range args {
			if a == "--body" && i+1 < len(args) {
				capturedBody = args[i+1]
			}
		}
		return []byte("https://github.com/org/repo/issues/2\n"), nil
	})
	defer SetGHCommandForTest(nil)

	result := &HealResult{
		FailReason: "2 candidates: 1 http_error, 1 binary_content",
		CandidateOutcomes: []CandidateOutcome{
			{URL: "https://example.com/missing.md", Strategy: "variant", Outcome: OutcomeHTTPError, StatusCode: 404, Detail: "status 404"},
			{URL: "https://example.com/image.md", Strategy: "variant", Outcome: OutcomeBinaryContent, StatusCode: 200, ContentType: "image/png"},
		},
	}
	if _, err := createHealFailureIssue("claude-code", "skills", 0, 2, result); err != nil {
		t.Fatalf("createHealFailureIssue: %v", err)
	}
	for _, want := range []string{
		"| Strategy | Candidate URL | Outcome | Status | Final URL | Detail |",
		"|---|---|---|---|---|---|",
		"https://example.com/missing.md",
		"http_error",
		"https://example.com/image.md",
		"binary_content",
	} {
		if !strings.Contains(capturedBody, want) {
			t.Errorf("issue body missing %q\n\nFull body:\n%s", want, capturedBody)
		}
	}
}

func TestRecordConsecutiveHealFailure_AppendUsesFailReason(t *testing.T) {
	// On second-and-later failures, the comment appended to the existing
	// issue must contain HealResult.FailReason so triagers can see what
	// the latest probe set looked like.
	cacheRoot := t.TempDir()
	anchor := HealFailureAnchor("claude-code", "skills", 0)
	var capturedComment string
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if args[0] == "issue" && args[1] == "list" {
			return []byte(`[{"number": 88, "body": "old ` + anchor + ` body"}]`), nil
		}
		if args[0] == "issue" && args[1] == "comment" {
			for i, a := range args {
				if a == "--body" && i+1 < len(args) {
					capturedComment = args[i+1]
				}
			}
			return nil, nil
		}
		return nil, nil
	})
	defer SetGHCommandForTest(nil)

	prev := healFailureCountFile(cacheRoot, "claude-code", "skills", 0)
	_ = os.MkdirAll(filepath.Dir(prev), 0755)
	_ = os.WriteFile(prev, []byte("2"), 0644) // already at threshold

	wantReason := "3 candidates: 2 http_error, 1 connect_error"
	if _, err := RecordConsecutiveHealFailure(cacheRoot, "claude-code", "skills", 0, &HealResult{FailReason: wantReason}); err != nil {
		t.Fatalf("RecordConsecutiveHealFailure: %v", err)
	}
	if !strings.Contains(capturedComment, wantReason) {
		t.Errorf("comment missing FailReason %q; got %q", wantReason, capturedComment)
	}
}

func TestHealFailureCountFile_PerSourceScoping(t *testing.T) {
	// Two sources in the same provider must map to distinct counter files
	// so fixing one doesn't reset the other.
	a := healFailureCountFile("/tmp/cache", "claude-code", "skills", 0)
	b := healFailureCountFile("/tmp/cache", "claude-code", "skills", 1)
	c := healFailureCountFile("/tmp/cache", "claude-code", "agents", 0)
	if a == b || a == c || b == c {
		t.Errorf("counter files collide: %q, %q, %q", a, b, c)
	}
}
