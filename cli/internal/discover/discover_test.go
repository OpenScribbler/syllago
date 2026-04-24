package discover

import (
	"testing"
)

func TestDiscoverMonolithicRules_Empty(t *testing.T) {
	tmp := t.TempDir()
	got, err := DiscoverMonolithicRules(tmp, "", []string{"CLAUDE.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 candidates, got %d: %+v", len(got), got)
	}
}
