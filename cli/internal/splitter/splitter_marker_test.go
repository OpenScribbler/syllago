package splitter

import (
	"strings"
	"testing"
)

func TestSplit_MarkerLiteral(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "marker-literal.md")
	cands, skip := Split(body, Options{
		Heuristic:     HeuristicMarker,
		MarkerLiteral: "===SYLLAGO-SPLIT===",
	})
	if skip != nil {
		t.Fatalf("unexpected skip-split: %+v", skip)
	}
	// Fixture contains 3 marker lines delimiting 4 regions.
	if len(cands) != 4 {
		t.Fatalf("expected 4 marker-separated candidates, got %d", len(cands))
	}
	// The literal marker heuristic does not produce a heading, so Name and
	// Description are empty; caller fills those in at review time.
	for i, c := range cands {
		if c.Name != "" {
			t.Errorf("cand %d: expected empty Name, got %q", i, c.Name)
		}
		if c.Description != "" {
			t.Errorf("cand %d: expected empty Description, got %q", i, c.Description)
		}
		// Marker lines must not appear inside any candidate body.
		if strings.Contains(c.Body, "===SYLLAGO-SPLIT===") {
			t.Errorf("cand %d body contains marker line (must be stripped)", i)
		}
	}
	// Specific region anchors — verify each expected region ended up in a
	// candidate (order-independent to avoid over-constraining empty-line trim).
	anchors := []string{"First region.", "Second region.", "Third region.", "Fourth region."}
	for _, anchor := range anchors {
		found := false
		for _, c := range cands {
			if strings.Contains(c.Body, anchor) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no candidate body contains anchor %q", anchor)
		}
	}
}

func TestSplit_MarkerLiteralEmptyMarkerReturnsNil(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "marker-literal.md")
	cands, skip := Split(body, Options{
		Heuristic:     HeuristicMarker,
		MarkerLiteral: "",
	})
	if skip != nil {
		t.Fatalf("unexpected skip-split: %+v", skip)
	}
	if cands != nil {
		t.Fatalf("expected nil candidates when marker literal is empty, got %d", len(cands))
	}
}
