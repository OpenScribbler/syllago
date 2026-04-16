package capmon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const healManifestFixture = `schema_version: "1"
slug: test-provider
display_name: Test Provider
# keep this comment — test asserts comment preservation
content_types:
  skills:
    sources:
      - url: "https://example.com/docs/old.md"
        type: documentation
        format: md
        selector: {}
      - url: "https://example.com/docs/other.md"
        type: documentation
        format: md
        selector: {}
`

func TestUpdateManifestURL_ReplacesTargetOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(healManifestFixture), 0644); err != nil {
		t.Fatal(err)
	}

	err := UpdateManifestURL(path, "skills", 0, "https://example.com/docs/old.md", "https://example.com/docs/new.md")
	if err != nil {
		t.Fatalf("UpdateManifestURL: %v", err)
	}

	out, _ := os.ReadFile(path)
	s := string(out)
	if !strings.Contains(s, "https://example.com/docs/new.md") {
		t.Errorf("new URL not present: %s", s)
	}
	if strings.Contains(s, "https://example.com/docs/old.md") {
		t.Errorf("old URL still present: %s", s)
	}
	if !strings.Contains(s, "https://example.com/docs/other.md") {
		t.Errorf("unrelated source lost its URL: %s", s)
	}
}

func TestUpdateManifestURL_WrongOldURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(healManifestFixture), 0644); err != nil {
		t.Fatal(err)
	}

	err := UpdateManifestURL(path, "skills", 0, "https://example.com/docs/WRONG.md", "https://example.com/docs/new.md")
	if err == nil {
		t.Fatal("expected error when expectedOldURL does not match")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("error should mention mismatch; got %v", err)
	}
}

func TestUpdateManifestURL_IndexOutOfRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(healManifestFixture), 0644); err != nil {
		t.Fatal(err)
	}

	err := UpdateManifestURL(path, "skills", 99, "https://example.com/docs/old.md", "https://example.com/docs/new.md")
	if err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}

func TestUpdateManifestURL_MissingContentType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(healManifestFixture), 0644); err != nil {
		t.Fatal(err)
	}

	err := UpdateManifestURL(path, "nonexistent", 0, "old", "new")
	if err == nil {
		t.Fatal("expected error for missing content type")
	}
}

func TestHealPRAnchor(t *testing.T) {
	got := HealPRAnchor("claude-code", "skills", 2)
	want := "<!-- capmon-heal: claude-code/skills/2 -->"
	if got != want {
		t.Errorf("HealPRAnchor = %q, want %q", got, want)
	}
}

func TestBuildHealPRBody(t *testing.T) {
	in := HealPRInputs{
		Provider:    "claude-code",
		ContentType: "skills",
		SourceIndex: 0,
		RunID:       "run-123",
		OldURL:      "https://example.com/old.md",
		Heal: HealResult{
			Success:   true,
			NewURL:    "https://example.com/new.md",
			Strategy:  "redirect",
			Proof:     "followed 1 permanent redirect",
			TriedURLs: []string{"https://example.com/old.md", "https://example.com/new.md"},
		},
	}
	body := BuildHealPRBody(in)
	// Must contain anchor for dedup
	if !strings.Contains(body, "<!-- capmon-heal: claude-code/skills/0 -->") {
		t.Error("PR body missing heal anchor")
	}
	// Must contain old URL, new URL, strategy
	if !strings.Contains(body, "https://example.com/old.md") {
		t.Error("body missing old URL")
	}
	if !strings.Contains(body, "https://example.com/new.md") {
		t.Error("body missing new URL")
	}
	if !strings.Contains(body, "redirect") {
		t.Error("body missing strategy")
	}
	if !strings.Contains(body, "run-123") {
		t.Error("body missing run ID")
	}
}

func TestFindOpenCapmonHealPR_Hit(t *testing.T) {
	anchor := HealPRAnchor("claude-code", "skills", 0)
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		// gh pr list --label capmon-heal --state open --json url,body
		return []byte(`[
			{"url": "https://github.com/org/repo/pull/100", "body": "unrelated PR"},
			{"url": "https://github.com/org/repo/pull/101", "body": "body with ` + anchor + ` inside"}
		]`), nil
	})
	defer SetGHCommandForTest(nil)

	url, found, err := FindOpenCapmonHealPR("claude-code", "skills", 0)
	if err != nil {
		t.Fatalf("FindOpenCapmonHealPR: %v", err)
	}
	if !found {
		t.Fatal("expected to find matching PR")
	}
	if url != "https://github.com/org/repo/pull/101" {
		t.Errorf("url = %q", url)
	}
}

func TestFindOpenCapmonHealPR_Miss(t *testing.T) {
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer SetGHCommandForTest(nil)

	_, found, err := FindOpenCapmonHealPR("claude-code", "skills", 0)
	if err != nil {
		t.Fatalf("FindOpenCapmonHealPR: %v", err)
	}
	if found {
		t.Error("expected no match")
	}
}

func TestProposeManifestHealPR_DedupesToExistingPR(t *testing.T) {
	anchor := HealPRAnchor("claude-code", "skills", 0)
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if args[0] == "pr" && args[1] == "list" {
			return []byte(`[{"url": "https://github.com/org/repo/pull/500", "body": "existing ` + anchor + `"}]`), nil
		}
		t.Errorf("unexpected gh call: %v", args)
		return nil, nil
	})
	defer SetGHCommandForTest(nil)

	// git runner should never be invoked because dedup short-circuits.
	SetGitRunnerForTest(func(dir string, args ...string) ([]byte, error) {
		t.Errorf("git should not be invoked on dedup hit; called with %v", args)
		return nil, nil
	})
	defer SetGitRunnerForTest(nil)

	url, err := ProposeManifestHealPR(context.Background(), t.TempDir(), HealPRInputs{
		Provider:    "claude-code",
		ContentType: "skills",
		SourceIndex: 0,
	})
	if err != nil {
		t.Fatalf("ProposeManifestHealPR: %v", err)
	}
	if url != "https://github.com/org/repo/pull/500" {
		t.Errorf("url = %q, want dedup target", url)
	}
}

func TestProposeManifestHealPR_FullFlow(t *testing.T) {
	// Stub gh: dedup returns empty, then PR create returns URL.
	var ghCalls [][]string
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		ghCalls = append(ghCalls, args)
		if args[0] == "pr" && args[1] == "list" {
			return []byte(`[]`), nil
		}
		if args[0] == "pr" && args[1] == "create" {
			return []byte("https://github.com/org/repo/pull/42\n"), nil
		}
		t.Errorf("unexpected gh: %v", args)
		return nil, nil
	})
	defer SetGHCommandForTest(nil)

	// Stub git: record calls, succeed every time.
	var gitCalls [][]string
	SetGitRunnerForTest(func(dir string, args ...string) ([]byte, error) {
		gitCalls = append(gitCalls, args)
		return nil, nil
	})
	defer SetGitRunnerForTest(nil)

	// Write a manifest in a temp repo dir so UpdateManifestURL has something
	// real to edit.
	repoDir := t.TempDir()
	manifestPath := filepath.Join(repoDir, "claude-code.yaml")
	if err := os.WriteFile(manifestPath, []byte(healManifestFixture), 0644); err != nil {
		t.Fatal(err)
	}

	url, err := ProposeManifestHealPR(context.Background(), repoDir, HealPRInputs{
		ManifestPath: manifestPath,
		Provider:     "claude-code",
		ContentType:  "skills",
		SourceIndex:  0,
		RunID:        "run-42",
		OldURL:       "https://example.com/docs/old.md",
		Heal: HealResult{
			NewURL:   "https://example.com/docs/new.md",
			Strategy: "redirect",
			Proof:    "followed 1 permanent redirect",
		},
	})
	if err != nil {
		t.Fatalf("ProposeManifestHealPR: %v", err)
	}
	if url != "https://github.com/org/repo/pull/42" {
		t.Errorf("url = %q", url)
	}

	// Expect: checkout, add, commit, push.
	wantGitOps := []string{"checkout", "add", "commit", "push"}
	if len(gitCalls) != len(wantGitOps) {
		t.Fatalf("git calls = %d, want %d: %v", len(gitCalls), len(wantGitOps), gitCalls)
	}
	for i, want := range wantGitOps {
		if gitCalls[i][0] != want {
			t.Errorf("gitCalls[%d][0] = %q, want %q", i, gitCalls[i][0], want)
		}
	}

	// Branch name should follow the scheme capmon/heal-<slug>/<ct>/<run-id>.
	checkoutArgs := gitCalls[0]
	if len(checkoutArgs) < 3 {
		t.Fatalf("checkout args: %v", checkoutArgs)
	}
	wantBranch := "capmon/heal-claude-code/skills/run-42"
	if checkoutArgs[2] != wantBranch {
		t.Errorf("branch = %q, want %q", checkoutArgs[2], wantBranch)
	}

	// Manifest file on disk should now carry the new URL.
	written, _ := os.ReadFile(manifestPath)
	if !strings.Contains(string(written), "new.md") {
		t.Error("manifest was not updated with new URL")
	}
}
