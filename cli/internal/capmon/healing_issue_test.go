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

	issueNum, err := RecordConsecutiveHealFailure(cacheRoot, "claude-code", "skills", 0, "variant: all 404")
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

	issueNum, err := RecordConsecutiveHealFailure(cacheRoot, "claude-code", "skills", 0, "github-rename: no candidates")
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

	issueNum, err := RecordConsecutiveHealFailure(cacheRoot, "claude-code", "skills", 0, "redirect: still 404")
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

	_, err := createHealFailureIssue("claude-code", "skills", 0, 2, "variant: nothing worked")
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
