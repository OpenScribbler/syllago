package splitter

import (
	"os"
	"path/filepath"
	"testing"
)

// fixturesDir is the shared test-data directory from Phase 1. Fixtures live
// under cli/internal/converter/testdata/splitter/ and are referenced from this
// package via a relative path.
const fixturesDir = "../converter/testdata/splitter"

// loadFixture reads a fixture file and returns its bytes. Fail the test on error.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	p := filepath.Join(fixturesDir, name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("load fixture %s: %v", p, err)
	}
	return b
}

func TestSplitCandidate_ZeroValue(t *testing.T) {
	t.Parallel()
	var sc SplitCandidate
	if sc.Name != "" {
		t.Fatalf("expected zero-value Name to be empty, got %q", sc.Name)
	}
	if sc.Description != "" {
		t.Fatalf("expected zero-value Description to be empty, got %q", sc.Description)
	}
	if sc.Body != "" {
		t.Fatalf("expected zero-value Body to be empty, got %q", sc.Body)
	}
	if sc.OriginalRange != [2]int{0, 0} {
		t.Fatalf("expected zero-value OriginalRange to be [0,0], got %v", sc.OriginalRange)
	}
}

func TestSplit_SkipSplitTooSmall(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "too-small.md")
	cands, skip := Split(body, Options{Heuristic: HeuristicH2})
	if cands != nil {
		t.Fatalf("expected nil candidates, got %d", len(cands))
	}
	if skip == nil {
		t.Fatal("expected non-nil SkipSplitSignal")
	}
	if skip.Reason != "too_small" {
		t.Fatalf("expected reason too_small, got %q", skip.Reason)
	}
}

func TestSplit_SkipSplitTooFewH2(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "no-h2.md")
	cands, skip := Split(body, Options{Heuristic: HeuristicH2})
	if cands != nil {
		t.Fatalf("expected nil candidates, got %d", len(cands))
	}
	if skip == nil {
		t.Fatal("expected non-nil SkipSplitSignal")
	}
	if skip.Reason != "too_few_h2" {
		t.Fatalf("expected reason too_few_h2, got %q", skip.Reason)
	}
}
