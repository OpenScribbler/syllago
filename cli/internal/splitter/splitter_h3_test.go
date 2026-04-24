package splitter

import (
	"bytes"
	"strings"
	"testing"
)

func TestSplit_H3Deep(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "h3-deep.md")
	cands, skip := Split(body, Options{Heuristic: HeuristicH3})
	if skip != nil {
		t.Fatalf("unexpected skip-split: %+v", skip)
	}
	// Count H3 headings in the fixture for expected candidate count.
	want := 0
	for _, ln := range bytes.Split(body, []byte{'\n'}) {
		if bytes.HasPrefix(ln, []byte("### ")) && !bytes.HasPrefix(ln, []byte("#### ")) {
			want++
		}
	}
	if want < 11 {
		t.Fatalf("test precondition: fixture must have at least 11 H3s, found %d", want)
	}
	if len(cands) != want {
		t.Fatalf("expected %d H3 candidates, got %d", want, len(cands))
	}
	// Every candidate body is header-promoted: leading line is "# <heading>".
	for i, c := range cands {
		if !strings.HasPrefix(c.Body, "# ") {
			t.Errorf("cand %d body missing promoted H1 prefix; got %q", i, c.Body[:min(16, len(c.Body))])
		}
	}
}
