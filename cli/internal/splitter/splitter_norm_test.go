package splitter

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
)

// TestSplit_NormalizationFixtures exercises the splitter against inputs run
// through canonical.Normalize (D12). The splitter itself does not normalize —
// callers normalize before calling Split. These tests assert that the
// normalization + splitter chain agrees on byte semantics for the four
// D12-motivated fixtures.
//
// Expected counts reflect each fixture's structural reality after normalization:
//   - crlf-line-endings.md (29 normalized lines, 4 H2) — below the 30-line floor,
//     so the splitter returns *SkipSplitSignal{too_small}. The CRLF-to-LF pass
//     already happened in Normalize, so the splitter sees LF-only bytes.
//   - bom-prefix.md (25 normalized lines, 4 H2) — below the 30-line floor →
//     SkipSplitSignal{too_small}. BOM already stripped by Normalize.
//   - no-trailing-newline.md (24 normalized lines, 4 H2) — below the 30-line
//     floor → SkipSplitSignal{too_small}. Trailing newline already added.
//   - trailing-whitespace.md (37 normalized lines, 4 H2) — splits to 4
//     candidates. The two-trailing-spaces line must survive byte-for-byte in
//     whichever section contains it.
func TestSplit_NormalizationFixtures(t *testing.T) {
	t.Parallel()

	t.Run("crlf-line-endings", func(t *testing.T) {
		t.Parallel()
		raw := loadFixture(t, "crlf-line-endings.md")
		cands, skip := Split(canonical.Normalize(raw), Options{Heuristic: HeuristicH2})
		if cands != nil {
			t.Fatalf("expected nil candidates, got %d", len(cands))
		}
		if skip == nil || skip.Reason != "too_small" {
			t.Fatalf("expected SkipSplitSignal{too_small}, got %+v", skip)
		}
	})

	t.Run("bom-prefix", func(t *testing.T) {
		t.Parallel()
		raw := loadFixture(t, "bom-prefix.md")
		cands, skip := Split(canonical.Normalize(raw), Options{Heuristic: HeuristicH2})
		if cands != nil {
			t.Fatalf("expected nil candidates, got %d", len(cands))
		}
		if skip == nil || skip.Reason != "too_small" {
			t.Fatalf("expected SkipSplitSignal{too_small}, got %+v", skip)
		}
	})

	t.Run("no-trailing-newline", func(t *testing.T) {
		t.Parallel()
		raw := loadFixture(t, "no-trailing-newline.md")
		cands, skip := Split(canonical.Normalize(raw), Options{Heuristic: HeuristicH2})
		if cands != nil {
			t.Fatalf("expected nil candidates, got %d", len(cands))
		}
		if skip == nil || skip.Reason != "too_small" {
			t.Fatalf("expected SkipSplitSignal{too_small}, got %+v", skip)
		}
	})

	t.Run("trailing-whitespace", func(t *testing.T) {
		t.Parallel()
		raw := loadFixture(t, "trailing-whitespace.md")
		cands, skip := Split(canonical.Normalize(raw), Options{Heuristic: HeuristicH2})
		if skip != nil {
			t.Fatalf("unexpected skip-split: %+v", skip)
		}
		if len(cands) != 4 {
			t.Fatalf("expected 4 candidates, got %d", len(cands))
		}
		// The fixture contains one line that ends with two spaces + LF (markdown
		// forced line break). Normalization does not strip trailing whitespace;
		// neither does the splitter. Assert the two-trailing-spaces line appears
		// byte-identical in the candidate whose range covers it.
		const twoSpaceLine = "This fixture intentionally contains a line with two trailing spaces.  "
		hits := 0
		for _, c := range cands {
			for _, ln := range strings.Split(c.Body, "\n") {
				if ln == twoSpaceLine {
					hits++
				}
			}
		}
		if hits != 1 {
			t.Fatalf("expected two-trailing-spaces line preserved in exactly one candidate, got %d matches", hits)
		}
	})
}
