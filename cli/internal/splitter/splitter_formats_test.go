package splitter

import (
	"bytes"
	"testing"
)

// TestSplit_FormatFixtures covers the five format-specific fixtures that
// replicate wire-format quirks of .cursorrules / .clinerules / .windsurfrules.
// Two are anti-fixtures (flat numbered lists with no H2 headings) that should
// trigger skip-split; one is a pointer stub; one is a clinerules-style file
// with numbered H2 headings; one is a cursorrules "see elsewhere" file with
// H2 subsections.
func TestSplit_FormatFixtures(t *testing.T) {
	t.Parallel()

	t.Run("cursorrules-flat-numbered", func(t *testing.T) {
		t.Parallel()
		cands, skip := Split(loadFixture(t, "cursorrules-flat-numbered.md"), Options{Heuristic: HeuristicH2})
		if cands != nil {
			t.Fatalf("expected nil candidates, got %d", len(cands))
		}
		if skip == nil || skip.Reason != "too_few_h2" {
			t.Fatalf("expected SkipSplitSignal{too_few_h2}, got %+v", skip)
		}
	})

	t.Run("windsurfrules-numbered-rules", func(t *testing.T) {
		t.Parallel()
		cands, skip := Split(loadFixture(t, "windsurfrules-numbered-rules.md"), Options{Heuristic: HeuristicH2})
		if cands != nil {
			t.Fatalf("expected nil candidates, got %d", len(cands))
		}
		if skip == nil || skip.Reason != "too_few_h2" {
			t.Fatalf("expected SkipSplitSignal{too_few_h2}, got %+v", skip)
		}
	})

	t.Run("windsurfrules-pointer", func(t *testing.T) {
		t.Parallel()
		cands, skip := Split(loadFixture(t, "windsurfrules-pointer.md"), Options{Heuristic: HeuristicH2})
		if cands != nil {
			t.Fatalf("expected nil candidates, got %d", len(cands))
		}
		if skip == nil || skip.Reason != "too_small" {
			t.Fatalf("expected SkipSplitSignal{too_small}, got %+v", skip)
		}
	})

	t.Run("clinerules-numbered-h2", func(t *testing.T) {
		t.Parallel()
		body := loadFixture(t, "clinerules-numbered-h2.md")
		cands, skip := Split(body, Options{Heuristic: HeuristicH2})
		if skip != nil {
			t.Fatalf("unexpected skip-split: %+v", skip)
		}
		if len(cands) != 5 {
			t.Fatalf("expected 5 candidates, got %d", len(cands))
		}
		// Numbered prefixes must be stripped from slugs; descriptions keep them.
		wantSlugs := []string{
			"project-overview",
			"coding-style",
			"testing",
			"error-handling",
			"release",
		}
		wantDescriptions := []string{
			"1. Project Overview",
			"2. Coding Style",
			"3. Testing",
			"4. Error Handling",
			"5. Release",
		}
		for i, c := range cands {
			if c.Name != wantSlugs[i] {
				t.Errorf("cand %d slug: want %q, got %q", i, wantSlugs[i], c.Name)
			}
			if c.Description != wantDescriptions[i] {
				t.Errorf("cand %d description: want %q, got %q", i, wantDescriptions[i], c.Description)
			}
		}
	})

	t.Run("cursorrules-points-elsewhere", func(t *testing.T) {
		t.Parallel()
		body := loadFixture(t, "cursorrules-points-elsewhere.md")
		cands, skip := Split(body, Options{Heuristic: HeuristicH2})
		if skip != nil {
			t.Fatalf("unexpected skip-split: %+v", skip)
		}
		// Expected candidate count equals the file's actual H2 count.
		wantH2 := 0
		for _, ln := range bytes.Split(body, []byte{'\n'}) {
			if bytes.HasPrefix(ln, []byte("## ")) && !bytes.HasPrefix(ln, []byte("### ")) {
				wantH2++
			}
		}
		if len(cands) != wantH2 {
			t.Fatalf("expected %d candidates matching H2 count, got %d", wantH2, len(cands))
		}
	})
}
