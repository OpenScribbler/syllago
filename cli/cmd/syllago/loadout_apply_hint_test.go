package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/loadout"
)

func TestLoadoutApplyHint_NamesEveryFlagTheRefusalNeeds(t *testing.T) {
	t.Parallel()
	unsupported := &loadout.UnsupportedHooksError{Provider: "Claude Code", Problems: []string{"a — x"}}
	flagged := &loadout.ScannerFindingsError{Problems: []string{"b — y"}}
	tests := []struct {
		name string
		err  error
		want []string
	}{
		{"unsupported", unsupported, []string{"--skip-unsupported"}},
		{"flagged", flagged, []string{"--force"}},
		{"both", errors.Join(unsupported, flagged), []string{"--skip-unsupported", "--force"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := loadoutApplyHint(tt.err)
			for _, flag := range tt.want {
				if !strings.Contains(hint, flag) {
					t.Errorf("hint %q does not name %s", hint, flag)
				}
			}
		})
	}
}
