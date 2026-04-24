package splitter

import (
	"strings"
	"testing"
)

func TestSplit_H2Clean(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "h2-clean.md")
	cands, skip := Split(body, Options{Heuristic: HeuristicH2})
	if skip != nil {
		t.Fatalf("unexpected skip-split: %+v", skip)
	}
	if len(cands) != 5 {
		t.Fatalf("expected 5 candidates, got %d", len(cands))
	}
	wantHeadings := []string{
		"Setup Instructions",
		"Project Structure",
		"Testing",
		"Deployment",
		"Contributing",
	}
	wantSlugs := []string{
		"setup-instructions",
		"project-structure",
		"testing",
		"deployment",
		"contributing",
	}
	// Zero-indexed start lines for H2 lines: 1, 11, 20, 30, 36.
	// End-exclusive boundaries: 11, 20, 30, 36, 42 (file has trailing newline).
	wantRanges := [][2]int{
		{1, 11},
		{11, 20},
		{20, 30},
		{30, 36},
		{36, 42},
	}
	for i, c := range cands {
		if c.Description != wantHeadings[i] {
			t.Errorf("cand %d description: want %q, got %q", i, wantHeadings[i], c.Description)
		}
		if c.Name != wantSlugs[i] {
			t.Errorf("cand %d name: want %q, got %q", i, wantSlugs[i], c.Name)
		}
		wantPrefix := "# " + wantHeadings[i] + "\n"
		if !strings.HasPrefix(c.Body, wantPrefix) {
			t.Errorf("cand %d body: expected prefix %q, got %q", i, wantPrefix, c.Body[:min(len(c.Body), len(wantPrefix)+16)])
		}
		if c.OriginalRange != wantRanges[i] {
			t.Errorf("cand %d range: want %v, got %v", i, wantRanges[i], c.OriginalRange)
		}
	}
}
