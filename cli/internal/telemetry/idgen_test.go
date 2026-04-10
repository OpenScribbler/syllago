package telemetry

import (
	"strings"
	"testing"
)

func TestGenerateID_Format(t *testing.T) {
	t.Parallel()
	id, err := generateID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(id, "syl_") {
		t.Errorf("ID missing syl_ prefix: %s", id)
	}
	if len(id) != 16 { // 4 + 12
		t.Errorf("unexpected ID length %d: %s", len(id), id)
	}
	if !isValidID(id) {
		t.Errorf("isValidID returned false for %s", id)
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := generateID()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestIsValidID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id    string
		valid bool
	}{
		{"syl_a1b2c3d4e5f6", true},
		{"syl_aabbccddeeff", true},
		{"syl_AABBCCDDEEFF", false},   // uppercase not valid
		{"syl_a1b2c3d4e5", false},     // too short
		{"syl_a1b2c3d4e5f6ff", false}, // too long
		{"abc_a1b2c3d4e5f6", false},   // wrong prefix
		{"", false},
	}
	for _, tc := range cases {
		if got := isValidID(tc.id); got != tc.valid {
			t.Errorf("isValidID(%q) = %v, want %v", tc.id, got, tc.valid)
		}
	}
}
