package splitter

import "testing"

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
