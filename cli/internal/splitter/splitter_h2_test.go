package splitter

import (
	"strings"
	"testing"
)

// preambleSnippet appears verbatim in h2-with-preamble.md's preamble region
// (lines before the first H2). Use it to assert preamble prepend behavior.
const preambleSnippet = "This document collects the working agreements for the repository"

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

func TestSplit_H2WithPreamble(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "h2-with-preamble.md")
	cands, skip := Split(body, Options{Heuristic: HeuristicH2})
	if skip != nil {
		t.Fatalf("unexpected skip-split: %+v", skip)
	}
	if len(cands) < 2 {
		t.Fatalf("expected >=2 candidates, got %d", len(cands))
	}
	// First candidate must contain preamble text below the promoted H1.
	if !strings.Contains(cands[0].Body, preambleSnippet) {
		t.Errorf("first candidate body missing preamble snippet %q; body:\n%s", preambleSnippet, cands[0].Body)
	}
	// Subsequent candidates must NOT contain the preamble snippet.
	for i := 1; i < len(cands); i++ {
		if strings.Contains(cands[i].Body, preambleSnippet) {
			t.Errorf("candidate %d unexpectedly contains preamble snippet", i)
		}
	}
	// The promoted H1 must still be first in the first candidate's body.
	wantFirstLine := "# " + cands[0].Description + "\n"
	if !strings.HasPrefix(cands[0].Body, wantFirstLine) {
		t.Errorf("first candidate does not start with promoted H1 %q; got:\n%s", wantFirstLine, cands[0].Body)
	}
}

func TestSplit_H2EmojiPrefix(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "h2-emoji-prefix.md")
	cands, skip := Split(body, Options{Heuristic: HeuristicH2})
	if skip != nil {
		t.Fatalf("unexpected skip-split: %+v", skip)
	}
	if len(cands) != 6 {
		t.Fatalf("expected 6 candidates, got %d", len(cands))
	}
	// Slugs must not contain emojis; emoji bytes fall outside [a-z0-9] and
	// should be replaced by the non-slug regex.
	wantSlugs := []string{
		"getting-started",
		"configuration",
		"building",
		"testing",
		"debugging",
		"documentation",
	}
	wantDescriptions := []string{
		"🚀 Getting Started",
		"🔧 Configuration",
		"📦 Building",
		"🧪 Testing",
		"🐛 Debugging",
		"📚 Documentation",
	}
	for i, c := range cands {
		if c.Name != wantSlugs[i] {
			t.Errorf("cand %d slug: want %q, got %q", i, wantSlugs[i], c.Name)
		}
		if c.Description != wantDescriptions[i] {
			t.Errorf("cand %d description: want %q, got %q", i, wantDescriptions[i], c.Description)
		}
		// Verify the emoji is preserved in the promoted H1 body prefix.
		if !strings.HasPrefix(c.Body, "# "+wantDescriptions[i]+"\n") {
			t.Errorf("cand %d body missing emoji-prefixed H1; got: %q", i, c.Body[:40])
		}
	}
}

func TestSplit_H2NumberedPrefix(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "h2-numbered-prefix.md")
	cands, skip := Split(body, Options{Heuristic: HeuristicH2})
	if skip != nil {
		t.Fatalf("unexpected skip-split: %+v", skip)
	}
	if len(cands) != 5 {
		t.Fatalf("expected 5 candidates, got %d", len(cands))
	}
	wantSlugs := []string{
		"coding-style",
		"error-handling",
		"testing-conventions",
		"documentation",
		"logging",
	}
	wantDescriptions := []string{
		"1. Coding Style",
		"2. Error Handling",
		"3. Testing Conventions",
		"4. Documentation",
		"5. Logging",
	}
	for i, c := range cands {
		if c.Name != wantSlugs[i] {
			t.Errorf("cand %d slug: want %q, got %q", i, wantSlugs[i], c.Name)
		}
		if c.Description != wantDescriptions[i] {
			t.Errorf("cand %d description: want %q, got %q", i, wantDescriptions[i], c.Description)
		}
	}
}

func TestSplit_H2ImportLine(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "import-line.md")
	cands, skip := Split(body, Options{Heuristic: HeuristicH2})
	if skip != nil {
		t.Fatalf("unexpected skip-split: %+v", skip)
	}
	if len(cands) == 0 {
		t.Fatalf("expected at least 1 candidate, got 0")
	}
	const importLine = "@import shared/rules.md"
	// Exactly one candidate must contain the @import line byte-for-byte on its
	// own line (the splitter must preserve the directive verbatim).
	hits := 0
	for _, c := range cands {
		for _, ln := range strings.Split(c.Body, "\n") {
			if ln == importLine {
				hits++
			}
		}
	}
	if hits != 1 {
		t.Fatalf("expected @import line preserved in exactly one candidate, got %d matches", hits)
	}
}

func TestSplit_H2DecorativeHR(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "decorative-hr.md")
	cands, skip := Split(body, Options{Heuristic: HeuristicH2})
	if skip != nil {
		t.Fatalf("unexpected skip-split: %+v", skip)
	}
	// Fixture has 4 H2s. Decorative --- lines must not create extra candidates.
	if len(cands) != 4 {
		t.Fatalf("expected 4 candidates (no splits on ---), got %d", len(cands))
	}
	// Every decorative --- in the source must still appear in one of the bodies.
	hrHits := 0
	for _, c := range cands {
		for _, ln := range strings.Split(c.Body, "\n") {
			if ln == "---" {
				hrHits++
			}
		}
	}
	// Source contains 4 standalone --- lines.
	if hrHits != 4 {
		t.Errorf("expected 4 --- lines preserved across candidate bodies, got %d", hrHits)
	}
}
