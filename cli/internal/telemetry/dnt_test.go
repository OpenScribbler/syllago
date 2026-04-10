package telemetry

import (
	"testing"
)

func TestIsDNTSet(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"ON", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"random", false},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("DO_NOT_TRACK", tc.env)
			if got := isDNTSet(); got != tc.want {
				t.Errorf("isDNTSet() with DO_NOT_TRACK=%q = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func TestIsDNTSet_Unset(t *testing.T) {
	// Ensure variable is absent, not just empty.
	// Cannot use t.Parallel() with t.Setenv().
	t.Setenv("DO_NOT_TRACK", "")
	if isDNTSet() {
		t.Error("expected false when DO_NOT_TRACK is unset/empty")
	}
}
