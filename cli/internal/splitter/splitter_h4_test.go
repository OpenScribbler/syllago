package splitter

import (
	"bytes"
	"testing"
)

func TestSplit_H4Rare(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "h4-rare.md")
	cands, skip := Split(body, Options{Heuristic: HeuristicH4})
	if skip != nil {
		t.Fatalf("unexpected skip-split: %+v", skip)
	}
	// Count H4 headings in the fixture for expected candidate count.
	want := 0
	for _, ln := range bytes.Split(body, []byte{'\n'}) {
		if bytes.HasPrefix(ln, []byte("#### ")) && !bytes.HasPrefix(ln, []byte("##### ")) {
			want++
		}
	}
	if want == 0 {
		t.Fatalf("test precondition: fixture must have at least one H4, found 0")
	}
	if len(cands) != want {
		t.Fatalf("expected %d H4 candidates, got %d", want, len(cands))
	}
}
